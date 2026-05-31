package service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

// Migration Test Framework
//
// This package tests schema migrations using checked-in test fixtures.
//
// IMPORTANT: When introducing schema vN, immediately add testdata/migrations/vN/
// with sample data in that format. Don't wait until vN+1 - add it now.
// TestMigrationFixturesComplete enforces this invariant.
//
// When adding new schema versions:
//  1. Create testdata/migrations/vN/ with sample data in the NEW format
//  2. Add tests that verify vN data doesn't need migration (like TestMigrateService_Plan_V1_NoChanges)
//  3. When vN+1 ships, add tests that migrate vN -> vN+1
//  4. Existing tests remain - ensuring all historical migrations still work
//
// Directory structure:
//
//	testdata/migrations/v0/  - Legacy data (no version stamps)
//	testdata/migrations/v1/  - Schema v1 data (labels as first-class)
//	testdata/migrations/v2/  - Schema v2 data (current, labels as custom field)
//	etc.

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// setupMigrationTest copies test fixtures to a temp directory and returns
// the MigrateService and cleanup function.
func setupMigrationTest(t *testing.T, fixtureName string) (*MigrateService, string, func()) {
	t.Helper()

	fixtureDir := filepath.Join("testdata", "migrations", fixtureName)
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Fatalf("Test fixture not found: %s", fixtureDir)
	}

	tempDir, err := os.MkdirTemp("", "kan-migrate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	if err := copyDir(fixtureDir, tempDir); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to copy test fixtures: %v", err)
	}

	paths := config.NewPaths(tempDir, "")
	service := NewMigrateService(paths)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return service, tempDir, cleanup
}

// ============================================================================
// Global config migration tests
// ============================================================================

// writeGlobalConfig writes raw TOML to the (HOME-isolated) global config path.
func writeGlobalConfig(t *testing.T, contents string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := config.GlobalConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write global config: %v", err)
	}
	return path
}

func TestMigrateService_GlobalConfig_V1ToCurrent(t *testing.T) {
	// A global/1 config with real content. Migration must bump the schema in
	// place (not prepend a duplicate kan_schema key) and preserve every field.
	path := writeGlobalConfig(t, `kan_schema = "global/1"
editor = "vim"

[projects]
myproj = "/home/me/myproj"

[repos."/home/me/myproj"]
default_board = "main"
data_location = ".kan"
`)

	svc := NewQuietMigrateService(config.NewPaths("", ""))
	plan, err := svc.PlanGlobalMigration()
	if err != nil {
		t.Fatalf("PlanGlobalMigration: %v", err)
	}
	if plan == nil || !plan.NeedsMigration {
		t.Fatalf("expected global/1 to need migration, got %+v", plan)
	}
	if plan.FromSchema != "global/1" {
		t.Errorf("FromSchema = %q, want global/1", plan.FromSchema)
	}

	if err := svc.Execute(&MigrationPlan{GlobalConfig: plan}, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// File must decode cleanly (no duplicate kan_schema) and carry the new
	// schema with all original fields intact.
	cfg, err := store.NewGlobalStore().Load()
	if err != nil {
		t.Fatalf("Load migrated config: %v", err)
	}
	if cfg.KanSchema != version.CurrentGlobalSchema() {
		t.Errorf("KanSchema = %q, want %q", cfg.KanSchema, version.CurrentGlobalSchema())
	}
	if cfg.Editor != "vim" {
		t.Errorf("Editor = %q, want vim", cfg.Editor)
	}
	if cfg.Projects["myproj"] != "/home/me/myproj" {
		t.Errorf("Projects not preserved: %+v", cfg.Projects)
	}
	if rc := cfg.GetRepoConfig("/home/me/myproj"); rc == nil || rc.DefaultBoard != "main" {
		t.Errorf("Repos not preserved: %+v", cfg.Repos)
	}

	// Guard against duplicate-key regression explicitly.
	data, _ := os.ReadFile(path)
	if strings.Count(string(data), "kan_schema") != 1 {
		t.Errorf("expected exactly one kan_schema key, file:\n%s", data)
	}
}

func TestMigrateService_GlobalConfig_Current_NoOp(t *testing.T) {
	writeGlobalConfig(t, fmt.Sprintf("kan_schema = %q\neditor = \"nano\"\n", version.CurrentGlobalSchema()))

	svc := NewQuietMigrateService(config.NewPaths("", ""))
	plan, err := svc.PlanGlobalMigration()
	if err != nil {
		t.Fatalf("PlanGlobalMigration: %v", err)
	}
	if plan != nil && plan.NeedsMigration {
		t.Errorf("current global schema should not need migration, got %+v", plan)
	}
}

// ============================================================================
// V0 -> V1 Migration Tests (Legacy data without version stamps)
// ============================================================================

func TestMigrateService_Plan_V0(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v0")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// Should detect board needs migration
	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if board.BoardName != "main" {
		t.Errorf("Expected board 'main', got %q", board.BoardName)
	}
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "" {
		t.Errorf("Expected empty FromSchema (missing), got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}

	// Should detect card needs migration
	if len(board.Cards) != 1 {
		t.Fatalf("Expected 1 card, got %d", len(board.Cards))
	}

	card := board.Cards[0]
	if card.CardID != "card-abc" {
		t.Errorf("Expected card 'card-abc', got %q", card.CardID)
	}
	if card.FromVersion != 0 {
		t.Errorf("Expected FromVersion 0 (missing), got %d", card.FromVersion)
	}
	if card.ToVersion != version.CurrentCardVersion {
		t.Errorf("Expected ToVersion %d, got %d", version.CurrentCardVersion, card.ToVersion)
	}
	if !card.RemoveColumn {
		t.Error("Card should have RemoveColumn=true (legacy cards have column field)")
	}
}

func TestMigrateService_Execute_V0(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v0")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// Execute migration (not dry run)
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify board config was migrated
	boardConfigPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	boardData, err := os.ReadFile(boardConfigPath)
	if err != nil {
		t.Fatalf("Failed to read migrated board config: %v", err)
	}

	var boardConfig map[string]any
	if err := toml.Unmarshal(boardData, &boardConfig); err != nil {
		t.Fatalf("Failed to parse migrated board config: %v", err)
	}

	if boardConfig["kan_schema"] != version.CurrentBoardSchema() {
		t.Errorf("Board config kan_schema = %v, want %q", boardConfig["kan_schema"], version.CurrentBoardSchema())
	}

	// Verify card was migrated
	cardPath := filepath.Join(tempDir, ".kan", "boards", "main", "cards", "card-abc.json")
	cardData, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("Failed to read migrated card: %v", err)
	}

	var cardJSON map[string]any
	if err := json.Unmarshal(cardData, &cardJSON); err != nil {
		t.Fatalf("Failed to parse migrated card: %v", err)
	}

	// Should have _v field
	if v, ok := cardJSON["_v"]; !ok {
		t.Error("Migrated card should have _v field")
	} else if int(v.(float64)) != version.CurrentCardVersion {
		t.Errorf("Card _v = %v, want %d", v, version.CurrentCardVersion)
	}

	// Should have column field (set by v9->v10 board migration)
	if _, ok := cardJSON["column"]; !ok {
		t.Error("Migrated card should have column field")
	}

	// Should have position field (set by v9->v10 board migration)
	if _, ok := cardJSON["position"]; !ok {
		t.Error("Migrated card should have position field")
	}

	// Should have seeded column history (card/3), even though the v9->v10 board
	// migration already stamped _v to the current version.
	history, ok := cardJSON["history"].([]any)
	if !ok || len(history) != 1 {
		t.Errorf("Migrated card should have one seeded history entry, got %v", cardJSON["history"])
	} else if entry := history[0].(map[string]any); entry["field"] != "column" {
		t.Errorf("Seeded history entry should track column, got %v", entry)
	}

	// Should preserve custom fields
	if cardJSON["priority"] != "high" {
		t.Errorf("Custom field 'priority' not preserved: %v", cardJSON["priority"])
	}
}

func TestMigrateService_Execute_DryRun(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v0")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// Execute with dry run
	if err := service.Execute(plan, true); err != nil {
		t.Fatalf("Execute (dry run) failed: %v", err)
	}

	// Verify files were NOT modified
	cardPath := filepath.Join(tempDir, ".kan", "boards", "main", "cards", "card-abc.json")
	cardData, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("Failed to read card: %v", err)
	}

	var cardJSON map[string]any
	if err := json.Unmarshal(cardData, &cardJSON); err != nil {
		t.Fatalf("Failed to parse card: %v", err)
	}

	// Should still have column field (not migrated)
	if _, ok := cardJSON["column"]; !ok {
		t.Error("Dry run should NOT modify files - column field should still exist")
	}

	// Should NOT have _v field (not migrated)
	if _, ok := cardJSON["_v"]; ok {
		t.Error("Dry run should NOT modify files - _v field should not exist")
	}
}

func TestMigrateService_Execute_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v0")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

func TestMigrateService_MigratedDataReadableByStore(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v0")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read migrated data
	paths := config.NewPaths(tempDir, "")
	cardStore := store.NewCardStore(paths)
	boardStore := store.NewBoardStore(paths)

	// Board store should read without error
	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}

	// Card store should read without error
	card, err := cardStore.Get("main", "card-abc")
	if err != nil {
		t.Fatalf("CardStore.Get failed after migration: %v", err)
	}
	if card.ID != "card-abc" {
		t.Errorf("Card ID = %q, want 'card-abc'", card.ID)
	}
	if card.Title != "Test Card" {
		t.Errorf("Card Title = %q, want 'Test Card'", card.Title)
	}

	// Custom fields should be preserved
	if card.CustomFields["priority"] != "high" {
		t.Errorf("Custom field 'priority' = %v, want 'high'", card.CustomFields["priority"])
	}
}

func TestMigrateService_V0ToCurrent_ConvertsLabels(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v0")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Read migrated config
	configPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	migratedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read migrated config: %v", err)
	}
	migratedContent := string(migratedData)

	// Should have current kan_schema
	expectedSchema := fmt.Sprintf(`kan_schema = %q`, version.CurrentBoardSchema())
	if !strings.Contains(migratedContent, expectedSchema) {
		t.Errorf("Migrated config should have %s", expectedSchema)
	}

	// Should have custom_fields.labels section
	if !strings.Contains(migratedContent, "[custom_fields]") {
		t.Error("Migrated config should have custom_fields section")
	}
	if !strings.Contains(migratedContent, "labels") {
		t.Error("Migrated config should have labels in custom_fields")
	}

	// Should have card_display section
	if !strings.Contains(migratedContent, "[card_display]") {
		t.Error("Migrated config should have card_display section")
	}

	// Should NOT have old [[labels]] section
	if strings.Contains(migratedContent, "[[labels]]") {
		t.Error("Migrated config should not have [[labels]] section")
	}
}

func TestMigrationPlan_HasChanges(t *testing.T) {
	tests := []struct {
		name     string
		plan     *MigrationPlan
		expected bool
	}{
		{
			name:     "empty plan",
			plan:     &MigrationPlan{},
			expected: false,
		},
		{
			name: "global config needs migration",
			plan: &MigrationPlan{
				GlobalConfig: &GlobalMigration{NeedsMigration: true},
			},
			expected: true,
		},
		{
			name: "board needs migration",
			plan: &MigrationPlan{
				Boards: []BoardMigration{{NeedsMigration: true}},
			},
			expected: true,
		},
		{
			name: "card needs migration",
			plan: &MigrationPlan{
				Boards: []BoardMigration{{
					Cards: []CardMigration{{FromVersion: 0, ToVersion: 1}},
				}},
			},
			expected: true,
		},
		{
			name: "nothing needs migration",
			plan: &MigrationPlan{
				GlobalConfig: &GlobalMigration{NeedsMigration: false},
				Boards: []BoardMigration{{
					NeedsMigration: false,
					Cards:          []CardMigration{},
				}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.plan.HasChanges(); got != tt.expected {
				t.Errorf("HasChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Fixture Completeness Test
// ============================================================================

// TestMigrationFixturesComplete ensures migration fixtures exist for all schema
// versions from 0 through the current version. This catches the case where
// someone bumps a version constant but forgets to add test fixtures.
//
// When you see this test fail:
//  1. Create testdata/migrations/vN/ with sample data in that schema format
//  2. Add tests that verify vN data doesn't need migration
//  3. See the header comment in this file for details
func TestMigrationFixturesComplete(t *testing.T) {
	// Fixtures should exist for v0 through the highest schema version.
	// Card, board, and global versions may diverge.
	maxVersion := version.CurrentCardVersion
	if version.CurrentBoardVersion > maxVersion {
		maxVersion = version.CurrentBoardVersion
	}
	if version.CurrentGlobalVersion > maxVersion {
		maxVersion = version.CurrentGlobalVersion
	}

	for v := 0; v <= maxVersion; v++ {
		fixturePath := filepath.Join("testdata", "migrations", fmt.Sprintf("v%d", v))
		if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
			t.Errorf("Missing migration fixtures for v%d at %s\n"+
				"When bumping schema versions, you must:\n"+
				"  1. Create testdata/migrations/v%d/ with sample data in that format\n"+
				"  2. Add tests that verify v%d data doesn't need migration\n"+
				"See migrate_service_test.go header comment for details.",
				v, fixturePath, v, v)
		}
	}
}

// ============================================================================
// V1 -> V2 Migration Tests (Labels conversion)
// ============================================================================

func TestMigrateService_Plan_V1_NeedsMigration(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v1")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// V1 should need migration to V2
	if !plan.HasChanges() {
		t.Error("V1 data should need migration to V2")
	}

	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "board/1" {
		t.Errorf("Expected FromSchema 'board/1', got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}
}

func TestMigrateService_V1ToCurrent_ConvertsLabels(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v1")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Read migrated config
	configPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	migratedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read migrated config: %v", err)
	}
	migratedContent := string(migratedData)

	// Should have current kan_schema
	expectedSchema := fmt.Sprintf(`kan_schema = %q`, version.CurrentBoardSchema())
	if !strings.Contains(migratedContent, expectedSchema) {
		t.Errorf("Migrated config should have %s, got:\n%s", expectedSchema, migratedContent)
	}

	// Should have custom_fields.labels section with tags type
	if !strings.Contains(migratedContent, "[custom_fields]") {
		t.Errorf("Migrated config should have custom_fields section, got:\n%s", migratedContent)
	}

	// Should NOT have old [[labels]] section
	if strings.Contains(migratedContent, "[[labels]]") {
		t.Errorf("Migrated config should not have [[labels]] section, got:\n%s", migratedContent)
	}

	// Should have card_display section with badges
	if !strings.Contains(migratedContent, "[card_display]") {
		t.Errorf("Migrated config should have card_display section, got:\n%s", migratedContent)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify labels custom field exists with correct type
	// After full migration chain (v1 -> v2 creates tags, v4 -> v5 renames to enum-set)
	labelsSchema, ok := boardCfg.CustomFields["labels"]
	if !ok {
		t.Fatal("Migrated board should have 'labels' custom field")
	}
	if labelsSchema.Type != "enum-set" {
		t.Errorf("labels custom field should be type 'enum-set', got %q", labelsSchema.Type)
	}

	// Verify the label value was preserved
	if len(labelsSchema.Options) != 1 {
		t.Fatalf("Expected 1 label option, got %d", len(labelsSchema.Options))
	}
	if labelsSchema.Options[0].Value != "bug" {
		t.Errorf("Expected label value 'bug', got %q", labelsSchema.Options[0].Value)
	}
	if labelsSchema.Options[0].Color != "#ef4444" {
		t.Errorf("Expected label color '#ef4444', got %q", labelsSchema.Options[0].Color)
	}

	// Verify card_display references labels
	if len(boardCfg.CardDisplay.Badges) != 1 || boardCfg.CardDisplay.Badges[0] != "labels" {
		t.Errorf("card_display.badges should be ['labels'], got %v", boardCfg.CardDisplay.Badges)
	}
}

func TestMigrateService_V1ToV2_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v1")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V2 -> V3 Migration Tests (pattern_hooks addition)
// ============================================================================

func TestMigrateService_Plan_V2_NeedsMigration(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v2")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// V2 should need migration to V3
	if !plan.HasChanges() {
		t.Error("V2 data should need migration to V3")
	}

	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "board/2" {
		t.Errorf("Expected FromSchema 'board/2', got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}
}

func TestMigrateService_V2ToCurrent_UpdatesSchema(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v2")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify schema was updated to current version
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected KanSchema %q, got %q", version.CurrentBoardSchema(), boardCfg.KanSchema)
	}

	// Existing fields should be preserved
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if len(boardCfg.CustomFields) == 0 {
		t.Error("CustomFields should be preserved")
	}
}

func TestMigrateService_V2ToV3_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v2")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V3 -> V4 Migration Tests (wanted fields addition)
// ============================================================================

func TestMigrateService_Plan_V3_NeedsMigration(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v3")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// V3 should need migration to V4
	if !plan.HasChanges() {
		t.Error("V3 data should need migration to V4")
	}

	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "board/3" {
		t.Errorf("Expected FromSchema 'board/3', got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}
}

func TestMigrateService_V3ToV4_UpdatesSchema(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v3")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify schema was updated to current
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected KanSchema %q, got %q", version.CurrentBoardSchema(), boardCfg.KanSchema)
	}

	// Existing fields should be preserved
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if len(boardCfg.CustomFields) == 0 {
		t.Error("CustomFields should be preserved")
	}
	if len(boardCfg.PatternHooks) != 1 {
		t.Error("PatternHooks should be preserved")
	}
}

func TestMigrateService_V3ToV4_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v3")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V4 -> V5 Migration Tests (tags renamed to enum-set)
// ============================================================================

func TestMigrateService_Plan_V4_NeedsMigration(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v4")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// V4 should need migration to V5
	if !plan.HasChanges() {
		t.Error("V4 data should need migration to V5")
	}

	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "board/4" {
		t.Errorf("Expected FromSchema 'board/4', got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}
}

func TestMigrateService_V4ToV5_RenamesTagsToEnumSet(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v4")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify schema was updated
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected KanSchema %q, got %q", version.CurrentBoardSchema(), boardCfg.KanSchema)
	}

	// Verify labels custom field type was renamed from "tags" to "enum-set"
	labelsSchema, ok := boardCfg.CustomFields["labels"]
	if !ok {
		t.Fatal("Migrated board should have 'labels' custom field")
	}
	if labelsSchema.Type != "enum-set" {
		t.Errorf("labels custom field should be type 'enum-set', got %q", labelsSchema.Type)
	}

	// Verify options were preserved
	if len(labelsSchema.Options) != 1 {
		t.Fatalf("Expected 1 label option, got %d", len(labelsSchema.Options))
	}
	if labelsSchema.Options[0].Value != "urgent" {
		t.Errorf("Expected label value 'urgent', got %q", labelsSchema.Options[0].Value)
	}

	// Verify other fields preserved
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	typeSchema, ok := boardCfg.CustomFields["type"]
	if !ok {
		t.Error("Expected 'type' custom field")
	} else if !typeSchema.Wanted {
		t.Error("Expected 'type' field to have wanted=true")
	}

	// Pattern hooks should be preserved
	if len(boardCfg.PatternHooks) != 1 {
		t.Errorf("Expected 1 pattern hook, got %d", len(boardCfg.PatternHooks))
	}
}

func TestMigrateService_V4ToV5_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v4")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V5 -> V6 Migration Tests (description fields added)
// ============================================================================

func TestMigrateService_Plan_V5_NeedsMigration(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v5")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// V5 should need migration to V6
	if !plan.HasChanges() {
		t.Error("V5 data should need migration to V6")
	}

	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "board/5" {
		t.Errorf("Expected FromSchema 'board/5', got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}
}

func TestMigrateService_V5ToV6_UpdatesSchema(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v5")
	defer cleanup()

	// Verify fixture starts at v5
	configPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	beforeData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config before migration: %v", err)
	}
	var beforeRaw map[string]any
	if err := toml.Unmarshal(beforeData, &beforeRaw); err != nil {
		t.Fatalf("Failed to parse config before migration: %v", err)
	}
	if beforeRaw["kan_schema"] != "board/5" {
		t.Fatalf("Expected fixture to start at board/5, got %v", beforeRaw["kan_schema"])
	}

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify schema was updated
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected KanSchema %q, got %q", version.CurrentBoardSchema(), boardCfg.KanSchema)
	}

	// Existing fields should be preserved
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if len(boardCfg.CustomFields) == 0 {
		t.Error("CustomFields should be preserved")
	}
	if len(boardCfg.PatternHooks) != 1 {
		t.Error("PatternHooks should be preserved")
	}

	// Wanted field should be preserved
	typeSchema, ok := boardCfg.CustomFields["type"]
	if !ok {
		t.Error("Expected 'type' custom field")
	} else if !typeSchema.Wanted {
		t.Error("Expected 'type' field to have wanted=true")
	}

	// enum-set field should be preserved
	labelsSchema, ok := boardCfg.CustomFields["labels"]
	if !ok {
		t.Error("Expected 'labels' custom field")
	} else if labelsSchema.Type != "enum-set" {
		t.Errorf("Expected labels type 'enum-set', got %q", labelsSchema.Type)
	}
}

func TestMigrateService_V5ToV6_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v5")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V6 -> V7 Migration Tests (column descriptions added)
// ============================================================================

func TestMigrateService_Plan_V6_NeedsMigration(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v6")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// V6 should need migration to V7
	if !plan.HasChanges() {
		t.Error("V6 data should need migration to V7")
	}

	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "board/6" {
		t.Errorf("Expected FromSchema 'board/6', got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}
}

func TestMigrateService_V6ToV7_UpdatesSchema(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v6")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify schema was updated
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected KanSchema %q, got %q", version.CurrentBoardSchema(), boardCfg.KanSchema)
	}

	// Existing fields should be preserved
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if len(boardCfg.CustomFields) == 0 {
		t.Error("CustomFields should be preserved")
	}
	if len(boardCfg.PatternHooks) != 1 {
		t.Error("PatternHooks should be preserved")
	}

	// Field descriptions should be preserved
	typeSchema, ok := boardCfg.CustomFields["type"]
	if !ok {
		t.Error("Expected 'type' custom field")
	} else if typeSchema.Description != "The category of work this card represents" {
		t.Errorf("Expected type description preserved, got %q", typeSchema.Description)
	}
}

func TestMigrateService_V6ToV7_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v6")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V7 -> V8 Tests (schema-only: adds optional limit to columns)
// ============================================================================

func TestMigrateService_Plan_V7_NeedsMigration(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v7")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if !plan.HasChanges() {
		t.Fatal("V7 data should need migration to v8")
	}

	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "board/7" {
		t.Errorf("Expected FromSchema 'board/7', got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}
}

func TestMigrateService_V7ToV8_UpdatesSchema(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v7")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify schema was updated
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected KanSchema %q, got %q", version.CurrentBoardSchema(), boardCfg.KanSchema)
	}

	// Existing fields should be preserved
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if len(boardCfg.CustomFields) == 0 {
		t.Error("CustomFields should be preserved")
	}
	if len(boardCfg.PatternHooks) != 1 {
		t.Error("PatternHooks should be preserved")
	}

	// Column description should be preserved
	backlog := boardCfg.GetColumn("Backlog")
	if backlog == nil {
		t.Fatal("Expected 'Backlog' column")
	}
	if backlog.Description != "Cards that are planned but not yet started" {
		t.Errorf("Expected Backlog description preserved, got %q", backlog.Description)
	}
}

func TestMigrateService_V7ToV8_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v7")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V8 -> V9 Tests (schema-only: adds boolean custom field type)
// ============================================================================

func TestMigrateService_Plan_V8_NeedsMigration(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v8")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if !plan.HasChanges() {
		t.Fatal("V8 data should need migration to v9")
	}

	if len(plan.Boards) != 1 {
		t.Fatalf("Expected 1 board, got %d", len(plan.Boards))
	}

	board := plan.Boards[0]
	if !board.NeedsMigration {
		t.Error("Board config should need migration")
	}
	if board.FromSchema != "board/8" {
		t.Errorf("Expected FromSchema 'board/8', got %q", board.FromSchema)
	}
	if board.ToSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected ToSchema %q, got %q", version.CurrentBoardSchema(), board.ToSchema)
	}
}

func TestMigrateService_V8ToV9_UpdatesSchema(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v8")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify schema was updated
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected KanSchema %q, got %q", version.CurrentBoardSchema(), boardCfg.KanSchema)
	}

	// Existing fields should be preserved
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if len(boardCfg.CustomFields) == 0 {
		t.Error("CustomFields should be preserved")
	}
	if len(boardCfg.PatternHooks) != 1 {
		t.Error("PatternHooks should be preserved")
	}

	// Column limit should be preserved
	backlog := boardCfg.GetColumn("Backlog")
	if backlog == nil {
		t.Fatal("Expected 'Backlog' column")
	}
	if backlog.Limit != 5 {
		t.Errorf("Expected Backlog Limit 5, got %d", backlog.Limit)
	}
}

func TestMigrateService_V8ToV9_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v8")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V10 Tests (board/10 -> board/11, schema-only bump for tint display slot)
// ============================================================================

func TestMigrateService_V10ToV11_UpdatesSchema(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v10")
	defer cleanup()

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if !plan.HasChanges() {
		t.Fatal("v10 data should need migration to v11")
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify stores can read the migrated data
	paths := config.NewPaths(tempDir, "")
	boardStore := store.NewBoardStore(paths)

	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed after migration: %v", err)
	}

	// Verify schema was updated
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Expected KanSchema %q, got %q", version.CurrentBoardSchema(), boardCfg.KanSchema)
	}

	// Existing fields should be preserved
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if len(boardCfg.CustomFields) == 0 {
		t.Error("CustomFields should be preserved")
	}
	if len(boardCfg.PatternHooks) != 1 {
		t.Error("PatternHooks should be preserved")
	}
}

func TestMigrateService_V10ToV11_Idempotent(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v10")
	defer cleanup()

	// First migration
	plan1, err := service.Plan()
	if err != nil {
		t.Fatalf("First Plan failed: %v", err)
	}
	if !plan1.HasChanges() {
		t.Fatal("First plan should have changes")
	}
	if err := service.Execute(plan1, false); err != nil {
		t.Fatalf("First Execute failed: %v", err)
	}

	// Second migration should be no-op
	plan2, err := service.Plan()
	if err != nil {
		t.Fatalf("Second Plan failed: %v", err)
	}
	if plan2.HasChanges() {
		t.Error("Second plan should have no changes (migration is idempotent)")
	}
}

// ============================================================================
// V11 Tests (Current schema - no migration needed)
// ============================================================================

func TestMigrateService_Plan_V11_NoChanges(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v11")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if plan.HasChanges() {
		t.Error("Current schema (v11) data should not need migration")
	}
}

func TestMigrateService_V11_ReadableByStores(t *testing.T) {
	_, tempDir, cleanup := setupMigrationTest(t, "v11")
	defer cleanup()

	// V11 fixtures should be directly readable by stores without migration
	paths := config.NewPaths(tempDir, "")
	cardStore := store.NewCardStore(paths)
	boardStore := store.NewBoardStore(paths)

	// Board store should read without error
	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed on v11 fixtures: %v", err)
	}
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Board KanSchema = %q, want %q", boardCfg.KanSchema, version.CurrentBoardSchema())
	}

	// Column description should be present
	backlog := boardCfg.GetColumn("Backlog")
	if backlog == nil {
		t.Fatal("Expected 'Backlog' column")
	}
	if backlog.Description != "Cards that are planned but not yet started" {
		t.Errorf("Expected Backlog description, got %q", backlog.Description)
	}

	// Column limit should be present
	if backlog.Limit != 5 {
		t.Errorf("Expected Backlog Limit 5, got %d", backlog.Limit)
	}

	// Done column should have no limit
	done := boardCfg.GetColumn("Done")
	if done == nil {
		t.Fatal("Expected 'Done' column")
	}
	if done.Limit != 0 {
		t.Errorf("Expected Done Limit 0 (no limit), got %d", done.Limit)
	}

	// Pattern hooks should be present
	if len(boardCfg.PatternHooks) != 1 {
		t.Errorf("Expected 1 pattern hook, got %d", len(boardCfg.PatternHooks))
	} else {
		hook := boardCfg.PatternHooks[0]
		if hook.Name != "jira-sync" {
			t.Errorf("Expected hook name 'jira-sync', got %q", hook.Name)
		}
		if hook.Timeout != 60 {
			t.Errorf("Expected hook timeout 60, got %d", hook.Timeout)
		}
	}

	// Wanted field should be present
	typeSchema, ok := boardCfg.CustomFields["type"]
	if !ok {
		t.Error("Expected 'type' custom field")
	} else {
		if !typeSchema.Wanted {
			t.Error("Expected 'type' field to have wanted=true")
		}
		if typeSchema.Description != "The category of work this card represents" {
			t.Errorf("Expected type description, got %q", typeSchema.Description)
		}
		if len(typeSchema.Options) < 2 {
			t.Fatalf("Expected at least 2 type options, got %d", len(typeSchema.Options))
		}
		if typeSchema.Options[0].Description != "A defect in existing functionality" {
			t.Errorf("Expected bug description, got %q", typeSchema.Options[0].Description)
		}
	}

	// enum-set field should be present
	labelsSchema, ok := boardCfg.CustomFields["labels"]
	if !ok {
		t.Error("Expected 'labels' custom field")
	} else if labelsSchema.Type != "enum-set" {
		t.Errorf("Expected labels type 'enum-set', got %q", labelsSchema.Type)
	}

	// free-set field should be present
	topicsSchema, ok := boardCfg.CustomFields["topics"]
	if !ok {
		t.Error("Expected 'topics' custom field")
	} else if topicsSchema.Type != "free-set" {
		t.Errorf("Expected topics type 'free-set', got %q", topicsSchema.Type)
	}

	// boolean field should be present
	hpSchema, ok := boardCfg.CustomFields["high_priority"]
	if !ok {
		t.Error("Expected 'high_priority' custom field")
	} else {
		if hpSchema.Type != "boolean" {
			t.Errorf("Expected high_priority type 'boolean', got %q", hpSchema.Type)
		}
		if !hpSchema.Wanted {
			t.Error("Expected 'high_priority' field to have wanted=true")
		}
	}

	// Tint field should be present (new in v11)
	tintSchema, ok := boardCfg.CustomFields["tint"]
	if !ok {
		t.Error("Expected 'tint' custom field")
	} else {
		if tintSchema.Type != "enum" {
			t.Errorf("Expected tint type 'enum', got %q", tintSchema.Type)
		}
		if len(tintSchema.Options) != 2 {
			t.Errorf("Expected 2 tint options, got %d", len(tintSchema.Options))
		}
	}

	// Tint display slot should be present
	if boardCfg.CardDisplay.Tint != "tint" {
		t.Errorf("Expected CardDisplay.Tint = 'tint', got %q", boardCfg.CardDisplay.Tint)
	}

	// Card store should read without error
	card, err := cardStore.Get("main", "card-abc")
	if err != nil {
		t.Fatalf("CardStore.Get failed on v11 fixtures: %v", err)
	}
	if card.ID != "card-abc" {
		t.Errorf("Card ID = %q, want 'card-abc'", card.ID)
	}
	if card.Version != version.CurrentCardVersion {
		t.Errorf("Card Version = %d, want %d", card.Version, version.CurrentCardVersion)
	}

	// Column and position should be present
	if card.Column != "Backlog" {
		t.Errorf("Card Column = %q, want 'Backlog'", card.Column)
	}
	if card.Position == "" {
		t.Error("Card Position should not be empty")
	}

	// Custom fields should be present
	if card.CustomFields["priority"] != "high" {
		t.Errorf("Custom field 'priority' = %v, want 'high'", card.CustomFields["priority"])
	}
	if card.CustomFields["high_priority"] != true {
		t.Errorf("Custom field 'high_priority' = %v, want true", card.CustomFields["high_priority"])
	}
	if card.CustomFields["tint"] != "red" {
		t.Errorf("Custom field 'tint' = %v, want 'red'", card.CustomFields["tint"])
	}
}

// ============================================================================
// Card v2 -> v3 Migration Tests (column history seeding)
// ============================================================================

func TestMigrateService_CardV2ToV3_SeedsHistory(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "card_v2_no_history")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if !plan.HasChanges() {
		t.Fatal("card/2 data should need migration to card/3")
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	cardPath := filepath.Join(tempDir, ".kan", "boards", "main", "cards", "card-abc.json")
	data, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("Failed to read migrated card: %v", err)
	}

	// Migration must produce the same compact one-line-per-entry history format
	// the store uses, so it doesn't write verbose multi-line history that then
	// reformats (a noisy diff) on the card's next edit.
	if !strings.Contains(string(data), `{"field":"column","value":"Done","at":1704307200000}`) {
		t.Errorf("migrated history not in compact one-line format:\n%s", data)
	}

	var card map[string]any
	if err := json.Unmarshal(data, &card); err != nil {
		t.Fatalf("Failed to parse migrated card: %v", err)
	}

	if int(card["_v"].(float64)) != version.CurrentCardVersion {
		t.Errorf("Card _v = %v, want %d", card["_v"], version.CurrentCardVersion)
	}

	history, ok := card["history"].([]any)
	if !ok || len(history) != 1 {
		t.Fatalf("Expected 1 seeded history entry, got %v", card["history"])
	}
	entry := history[0].(map[string]any)
	if entry["field"] != "column" {
		t.Errorf("Seeded entry field = %v, want 'column'", entry["field"])
	}
	if entry["value"] != "Done" {
		t.Errorf("Seeded entry value = %v, want current column 'Done'", entry["value"])
	}
	// Seeded at creation time (approximation for pre-existing cards).
	if int64(entry["at"].(float64)) != 1704307200000 {
		t.Errorf("Seeded entry at = %v, want created_at_millis", entry["at"])
	}

	// The migrated card must be readable by the (strict-version) store.
	paths := config.NewPaths(tempDir, "")
	cardStore := store.NewCardStore(paths)
	got, err := cardStore.Get("main", "card-abc")
	if err != nil {
		t.Fatalf("CardStore.Get failed after migration: %v", err)
	}
	if len(got.History) != 1 || got.History[0].Value != "Done" {
		t.Errorf("Store-read history = %+v, want single Done entry", got.History)
	}
}

func TestMigrateService_CardV3_NoOp(t *testing.T) {
	// The v11 fixture card is already card/3 with history; it must not be
	// re-migrated or have its history altered.
	service, _, cleanup := setupMigrationTest(t, "v11")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if plan.HasChanges() {
		t.Error("card/3 data with history should not need migration")
	}
}

func TestSeedCardHistory(t *testing.T) {
	t.Run("seeds from current column at creation time", func(t *testing.T) {
		raw := map[string]any{
			"column":            "review",
			"created_at_millis": float64(1700000000000),
		}
		if !seedCardHistory(raw) {
			t.Fatal("expected seedCardHistory to report a change")
		}
		history := raw["history"].([]map[string]any)
		if len(history) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(history))
		}
		if history[0]["field"] != "column" || history[0]["value"] != "review" {
			t.Errorf("unexpected entry: %v", history[0])
		}
		if history[0]["at"].(int64) != 1700000000000 {
			t.Errorf("expected at = created_at, got %v", history[0]["at"])
		}
	})

	t.Run("idempotent when history already present", func(t *testing.T) {
		raw := map[string]any{
			"column":  "review",
			"history": []any{map[string]any{"field": "column", "value": "backlog", "at": float64(1)}},
		}
		if seedCardHistory(raw) {
			t.Error("expected no change when history already present")
		}
	})

	t.Run("seeds when history is null or empty", func(t *testing.T) {
		for name, h := range map[string]any{"null": nil, "empty": []any{}} {
			raw := map[string]any{"column": "review", "created_at_millis": float64(1), "history": h}
			if !seedCardHistory(raw) {
				t.Errorf("%s history should be treated as absent and seeded", name)
			}
		}
	})

	t.Run("no-op when column absent", func(t *testing.T) {
		raw := map[string]any{"created_at_millis": float64(1)}
		if seedCardHistory(raw) {
			t.Error("expected no change when column missing")
		}
		if _, has := raw["history"]; has {
			t.Error("should not add history without a column")
		}
	})
}
