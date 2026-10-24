package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/version"
)

// setupAutoMigrateProject creates a temp project directory with a board at the
// given schema version and a card at the given version. Returns the Paths and
// a cleanup function.
func setupAutoMigrateProject(t *testing.T, boardSchema string, cardVersion int) (*config.Paths, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "kan-auto-migrate-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	kanDir := filepath.Join(tempDir, ".kan")
	boardDir := filepath.Join(kanDir, "boards", "main")
	cardsDir := filepath.Join(boardDir, "cards")
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create dirs: %v", err)
	}

	// Write board config
	boardConfig := map[string]any{
		"name":           "main",
		"id":             "test-board",
		"default_column": "Backlog",
		"columns": []map[string]any{
			{"name": "Backlog", "color": "#6b7280"},
			{"name": "Done", "color": "#10b981"},
		},
	}
	if boardSchema != "" {
		boardConfig["kan_schema"] = boardSchema
	}

	configPath := filepath.Join(boardDir, "config.toml")
	f, err := os.Create(configPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create config: %v", err)
	}
	if err := toml.NewEncoder(f).Encode(boardConfig); err != nil {
		f.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write config: %v", err)
	}
	f.Close()

	// Write card
	card := map[string]any{
		"id":                "card-1",
		"title":             "Test Card",
		"column":            "Backlog",
		"position":          "V",
		"creator":           "tester",
		"created_at_millis": 1704307200000,
		"updated_at_millis": 1704307200000,
	}
	if cardVersion > 0 {
		card["_v"] = cardVersion
	}

	cardData, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to marshal card: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cardsDir, "card-1.json"), cardData, 0644); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write card: %v", err)
	}

	paths := config.NewPaths(tempDir, "")
	return paths, func() { os.RemoveAll(tempDir) }
}

func TestAutoMigrateProject_CurrentVersion_NoOp(t *testing.T) {
	paths, cleanup := setupAutoMigrateProject(t, version.CurrentBoardSchema(), version.CurrentCardVersion)
	defer cleanup()

	err := autoMigrateProject(paths)
	if err != nil {
		t.Fatalf("autoMigrateProject should succeed for current version, got: %v", err)
	}
}

func TestAutoMigrateProject_OldVersion_Migrates(t *testing.T) {
	// Use board/9 which requires migration to board/10
	paths, cleanup := setupAutoMigrateProject(t, "board/9", version.CurrentCardVersion)
	defer cleanup()

	err := autoMigrateProject(paths)
	if err != nil {
		t.Fatalf("autoMigrateProject should succeed for old version, got: %v", err)
	}

	// Verify the board config was migrated
	configPath := paths.BoardConfigPath("main")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}
	schema, _ := raw["kan_schema"].(string)
	if schema != version.CurrentBoardSchema() {
		t.Errorf("Expected schema %q after migration, got %q", version.CurrentBoardSchema(), schema)
	}
}

func TestAutoMigrateProject_FutureBoardVersion_ReturnsError(t *testing.T) {
	futureSchema := version.FormatBoardSchema(version.CurrentBoardVersion + 1)
	paths, cleanup := setupAutoMigrateProject(t, futureSchema, version.CurrentCardVersion)
	defer cleanup()

	err := autoMigrateProject(paths)
	if err == nil {
		t.Fatal("autoMigrateProject should fail for future board version")
	}

	var schemaErr *version.SchemaVersionError
	if !errors.As(err, &schemaErr) {
		t.Errorf("Expected SchemaVersionError, got: %T: %v", err, err)
	}
}

func TestAutoMigrateProject_FutureCardVersion_ReturnsError(t *testing.T) {
	// Board is current but card is future
	paths, cleanup := setupAutoMigrateProject(t, version.CurrentBoardSchema(), version.CurrentCardVersion+1)
	defer cleanup()

	err := autoMigrateProject(paths)
	if err == nil {
		t.Fatal("autoMigrateProject should fail for future card version")
	}

	var schemaErr *version.SchemaVersionError
	if !errors.As(err, &schemaErr) {
		t.Errorf("Expected SchemaVersionError, got: %T: %v", err, err)
	}
}

func TestAutoMigrateProject_MissingSchema_Migrates(t *testing.T) {
	// Empty board schema (v0 - no kan_schema field)
	paths, cleanup := setupAutoMigrateProject(t, "", 0)
	defer cleanup()

	err := autoMigrateProject(paths)
	if err != nil {
		t.Fatalf("autoMigrateProject should succeed for missing schema, got: %v", err)
	}

	// Verify migration happened
	configPath := paths.BoardConfigPath("main")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}
	schema, _ := raw["kan_schema"].(string)
	if schema != version.CurrentBoardSchema() {
		t.Errorf("Expected schema %q after migration, got %q", version.CurrentBoardSchema(), schema)
	}
}

func writeProjectConfig(t *testing.T, paths *config.Paths, contents string) {
	t.Helper()
	if err := os.WriteFile(paths.ProjectConfigPath(), []byte(contents), 0644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
}

func TestAutoMigrateProject_OldProjectConfig_Migrates(t *testing.T) {
	paths, cleanup := setupAutoMigrateProject(t, version.CurrentBoardSchema(), version.CurrentCardVersion)
	defer cleanup()
	writeProjectConfig(t, paths, "kan_schema = \"project/1\"\nid = \"p_1\"\nname = \"old\"\n")

	if err := autoMigrateProject(paths); err != nil {
		t.Fatalf("autoMigrateProject: %v", err)
	}

	data, err := os.ReadFile(paths.ProjectConfigPath())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw["kan_schema"] != version.CurrentProjectSchema() {
		t.Errorf("kan_schema = %v, want %s", raw["kan_schema"], version.CurrentProjectSchema())
	}
}

func TestAutoMigrateProject_FutureProjectConfig_ReturnsError(t *testing.T) {
	paths, cleanup := setupAutoMigrateProject(t, version.CurrentBoardSchema(), version.CurrentCardVersion)
	defer cleanup()
	future := "kan_schema = \"project/999\"\nid = \"p_1\"\n"
	writeProjectConfig(t, paths, future)

	err := autoMigrateProject(paths)
	var schemaErr *version.SchemaVersionError
	if !errors.As(err, &schemaErr) {
		t.Fatalf("autoMigrateProject = %v, want SchemaVersionError", err)
	}

	data, _ := os.ReadFile(paths.ProjectConfigPath())
	if string(data) != future {
		t.Errorf("future project config was rewritten:\n%s", data)
	}
}
