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

	// Should NOT have column field
	if _, ok := cardJSON["column"]; ok {
		t.Error("Migrated card should NOT have column field")
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
	// Fixtures should exist for v0 through current version.
	// All schema versions (card, board, global) are currently in sync.
	// If they diverge, use max(CardVersion, BoardVersion, GlobalVersion).
	maxVersion := version.CurrentCardVersion

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
	labelsSchema, ok := boardCfg.CustomFields["labels"]
	if !ok {
		t.Fatal("Migrated board should have 'labels' custom field")
	}
	if labelsSchema.Type != "tags" {
		t.Errorf("labels custom field should be type 'tags', got %q", labelsSchema.Type)
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

	// Verify schema was updated to board/4
	if boardCfg.KanSchema != "board/4" {
		t.Errorf("Expected KanSchema 'board/4', got %q", boardCfg.KanSchema)
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
// V4 Tests (Current schema - no migration needed)
// ============================================================================

func TestMigrateService_Plan_V4_NoChanges(t *testing.T) {
	service, _, cleanup := setupMigrationTest(t, "v4")
	defer cleanup()

	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if plan.HasChanges() {
		t.Error("Current schema (v4) data should not need migration")
	}
}

func TestMigrateService_V4_ReadableByStores(t *testing.T) {
	_, tempDir, cleanup := setupMigrationTest(t, "v4")
	defer cleanup()

	// V4 fixtures should be directly readable by stores without migration
	paths := config.NewPaths(tempDir, "")
	cardStore := store.NewCardStore(paths)
	boardStore := store.NewBoardStore(paths)

	// Board store should read without error
	boardCfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get failed on v4 fixtures: %v", err)
	}
	if boardCfg.Name != "main" {
		t.Errorf("Board name = %q, want 'main'", boardCfg.Name)
	}
	if boardCfg.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("Board KanSchema = %q, want %q", boardCfg.KanSchema, version.CurrentBoardSchema())
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
	} else if !typeSchema.Wanted {
		t.Error("Expected 'type' field to have wanted=true")
	}

	// Card store should read without error
	card, err := cardStore.Get("main", "card-abc")
	if err != nil {
		t.Fatalf("CardStore.Get failed on v4 fixtures: %v", err)
	}
	if card.ID != "card-abc" {
		t.Errorf("Card ID = %q, want 'card-abc'", card.ID)
	}
	if card.Version != version.CurrentCardVersion {
		t.Errorf("Card Version = %d, want %d", card.Version, version.CurrentCardVersion)
	}

	// Custom fields should be present
	if card.CustomFields["priority"] != "high" {
		t.Errorf("Custom field 'priority' = %v, want 'high'", card.CustomFields["priority"])
	}
}
