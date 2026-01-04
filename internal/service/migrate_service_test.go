package service

import (
	"encoding/json"
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
// When adding new schema versions:
//   1. Create testdata/migrations/vN/ with sample data in the OLD format
//   2. Add tests that verify migration from vN to current version
//   3. Existing tests remain - ensuring all historical migrations still work
//
// Directory structure:
//   testdata/migrations/v0/  - Legacy data (no version stamps)
//   testdata/migrations/v1/  - Schema v1 data (when v2 is released)
//   etc.

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

func TestMigrateService_PreservesTOMLFormatting(t *testing.T) {
	service, tempDir, cleanup := setupMigrationTest(t, "v0")
	defer cleanup()

	// Read original board config
	configPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	originalData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read original config: %v", err)
	}
	originalContent := string(originalData)

	// Migrate
	plan, err := service.Plan()
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if err := service.Execute(plan, false); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Read migrated config
	migratedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read migrated config: %v", err)
	}
	migratedContent := string(migratedData)

	// Should have kan_schema prepended
	if !strings.HasPrefix(migratedContent, "kan_schema = ") {
		t.Error("Migrated config should start with kan_schema")
	}

	// Original content should be preserved after the prepended field
	// (with a blank line separator)
	if !strings.Contains(migratedContent, originalContent) {
		t.Error("Original config content should be preserved after kan_schema")
	}

	// Comments should be preserved (our test fixture has a comment)
	if !strings.Contains(migratedContent, "# Board config WITHOUT kan_schema") {
		t.Error("Comments should be preserved during migration")
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
