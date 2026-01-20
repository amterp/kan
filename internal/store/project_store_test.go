package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/version"
)

func setupTestProjectStore(t *testing.T) (*FileProjectStore, string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "kan-project-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create .kan directory
	kanDir := filepath.Join(dir, ".kan")
	if err := os.MkdirAll(kanDir, 0755); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create .kan dir: %v", err)
	}

	paths := config.NewPaths(dir, "")
	store := NewProjectStore(paths)

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return store, dir, cleanup
}

func TestFileProjectStore_SaveAndLoad(t *testing.T) {
	store, _, cleanup := setupTestProjectStore(t)
	defer cleanup()

	cfg := &model.ProjectConfig{
		ID:   "p_test123",
		Name: "Test Project",
		Favicon: model.FaviconConfig{
			Background: "#ef4444",
			IconType:   model.IconTypeLetter,
			Letter:     "T",
		},
	}

	// Save
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != cfg.ID {
		t.Errorf("ID mismatch: got %q, want %q", loaded.ID, cfg.ID)
	}
	if loaded.Name != cfg.Name {
		t.Errorf("Name mismatch: got %q, want %q", loaded.Name, cfg.Name)
	}
	if loaded.Favicon.Background != cfg.Favicon.Background {
		t.Errorf("Favicon.Background mismatch: got %q, want %q", loaded.Favicon.Background, cfg.Favicon.Background)
	}
	if loaded.KanSchema != version.CurrentProjectSchema() {
		t.Errorf("KanSchema mismatch: got %q, want %q", loaded.KanSchema, version.CurrentProjectSchema())
	}
}

func TestFileProjectStore_LoadReturnsDefaultsWhenMissing(t *testing.T) {
	store, _, cleanup := setupTestProjectStore(t)
	defer cleanup()

	// Load without creating file first
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should return empty defaults, not nil
	if cfg == nil {
		t.Fatal("Expected non-nil config for missing file")
	}
	if cfg.Name != "" {
		t.Errorf("Expected empty name, got %q", cfg.Name)
	}
}

func TestFileProjectStore_Exists(t *testing.T) {
	store, _, cleanup := setupTestProjectStore(t)
	defer cleanup()

	// Should not exist initially
	if store.Exists() {
		t.Error("Expected Exists() to return false before Save()")
	}

	// Save a config
	cfg := &model.ProjectConfig{
		ID:   "p_test",
		Name: "Test",
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Should exist now
	if !store.Exists() {
		t.Error("Expected Exists() to return true after Save()")
	}
}

// ============================================================================
// EnsureInitialized Tests
// ============================================================================

func TestFileProjectStore_EnsureInitialized_CreatesNewConfig(t *testing.T) {
	store, dir, cleanup := setupTestProjectStore(t)
	defer cleanup()

	// EnsureInitialized on a directory with no config
	if err := store.EnsureInitialized("my-project"); err != nil {
		t.Fatalf("EnsureInitialized failed: %v", err)
	}

	// Should now exist
	if !store.Exists() {
		t.Fatal("Config should exist after EnsureInitialized")
	}

	// Load and verify
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should have an ID
	if cfg.ID == "" {
		t.Error("Config should have a generated ID")
	}
	if !strings.HasPrefix(cfg.ID, "p_") {
		t.Errorf("ID should have p_ prefix, got %q", cfg.ID)
	}

	// Should have the provided name
	if cfg.Name != "my-project" {
		t.Errorf("Name = %q, want 'my-project'", cfg.Name)
	}

	// Should have favicon config derived from ID
	if cfg.Favicon.Background == "" {
		t.Error("Favicon should have a background color")
	}
	if cfg.Favicon.Letter != "M" {
		t.Errorf("Favicon letter = %q, want 'M' (from 'my-project')", cfg.Favicon.Letter)
	}

	// Verify the file on disk has correct schema
	path := filepath.Join(dir, ".kan", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}
	if !strings.Contains(string(data), `kan_schema = "project/1"`) {
		t.Error("Config file should have project/1 schema stamp")
	}
}

func TestFileProjectStore_EnsureInitialized_AddsIDToExistingConfig(t *testing.T) {
	store, dir, cleanup := setupTestProjectStore(t)
	defer cleanup()

	// Manually create a config file WITHOUT an ID (simulates old project)
	configPath := filepath.Join(dir, ".kan", "config.toml")
	oldConfig := `kan_schema = "project/1"
name = "old-project"

[favicon]
background = "#3b82f6"
icon_type = "letter"
letter = "O"
`
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatalf("Failed to write old config: %v", err)
	}

	// EnsureInitialized should add ID
	if err := store.EnsureInitialized("default-name"); err != nil {
		t.Fatalf("EnsureInitialized failed: %v", err)
	}

	// Load and verify
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should now have an ID
	if cfg.ID == "" {
		t.Error("Config should have a generated ID after EnsureInitialized")
	}

	// Should preserve the original name (not use default-name)
	if cfg.Name != "old-project" {
		t.Errorf("Name = %q, want 'old-project' (should preserve original)", cfg.Name)
	}

	// Should preserve the original favicon config (not regenerate)
	if cfg.Favicon.Background != "#3b82f6" {
		t.Errorf("Favicon.Background = %q, want '#3b82f6' (should preserve original)", cfg.Favicon.Background)
	}
	if cfg.Favicon.Letter != "O" {
		t.Errorf("Favicon.Letter = %q, want 'O' (should preserve original)", cfg.Favicon.Letter)
	}
}

func TestFileProjectStore_EnsureInitialized_NoOpIfIDExists(t *testing.T) {
	store, dir, cleanup := setupTestProjectStore(t)
	defer cleanup()

	// Create a config WITH an ID
	cfg := &model.ProjectConfig{
		ID:   "p_existing123",
		Name: "existing-project",
		Favicon: model.FaviconConfig{
			Background: "#ef4444",
			IconType:   model.IconTypeLetter,
			Letter:     "E",
		},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Get the file mod time
	configPath := filepath.Join(dir, ".kan", "config.toml")
	statBefore, _ := os.Stat(configPath)

	// EnsureInitialized should be a no-op
	if err := store.EnsureInitialized("different-name"); err != nil {
		t.Fatalf("EnsureInitialized failed: %v", err)
	}

	// File should not be modified (same mod time)
	statAfter, _ := os.Stat(configPath)
	if !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Error("EnsureInitialized should not modify file when ID already exists")
	}

	// Load and verify nothing changed
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != "p_existing123" {
		t.Errorf("ID = %q, want 'p_existing123' (should not change)", loaded.ID)
	}
	if loaded.Name != "existing-project" {
		t.Errorf("Name = %q, want 'existing-project' (should not change)", loaded.Name)
	}
}

func TestFileProjectStore_EnsureInitialized_GeneratesFaviconIfEmpty(t *testing.T) {
	store, dir, cleanup := setupTestProjectStore(t)
	defer cleanup()

	// Manually create a config WITHOUT an ID and WITHOUT favicon config
	configPath := filepath.Join(dir, ".kan", "config.toml")
	oldConfig := `kan_schema = "project/1"
name = "bare-project"
`
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatalf("Failed to write old config: %v", err)
	}

	// EnsureInitialized should add ID and generate favicon
	if err := store.EnsureInitialized("default-name"); err != nil {
		t.Fatalf("EnsureInitialized failed: %v", err)
	}

	// Load and verify
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Should have ID
	if cfg.ID == "" {
		t.Error("Config should have a generated ID")
	}

	// Should have favicon generated from the new ID
	if cfg.Favicon.Background == "" {
		t.Error("Favicon should have a generated background color")
	}
	if cfg.Favicon.Letter != "B" {
		t.Errorf("Favicon.Letter = %q, want 'B' (from 'bare-project')", cfg.Favicon.Letter)
	}
}

func TestFileProjectStore_EnsureInitialized_Idempotent(t *testing.T) {
	store, _, cleanup := setupTestProjectStore(t)
	defer cleanup()

	// First call
	if err := store.EnsureInitialized("test-project"); err != nil {
		t.Fatalf("First EnsureInitialized failed: %v", err)
	}

	cfg1, _ := store.Load()
	id1 := cfg1.ID

	// Second call should not change anything
	if err := store.EnsureInitialized("different-name"); err != nil {
		t.Fatalf("Second EnsureInitialized failed: %v", err)
	}

	cfg2, _ := store.Load()
	if cfg2.ID != id1 {
		t.Errorf("ID changed after second EnsureInitialized: %q -> %q", id1, cfg2.ID)
	}
	if cfg2.Name != "test-project" {
		t.Errorf("Name changed after second EnsureInitialized: want 'test-project', got %q", cfg2.Name)
	}
}

func TestColorFromID_Deterministic(t *testing.T) {
	// Same ID should always produce same color
	id := "p_test123abc"

	color1 := model.ColorFromID(id)
	color2 := model.ColorFromID(id)

	if color1 != color2 {
		t.Errorf("ColorFromID not deterministic: %q vs %q", color1, color2)
	}
}

func TestColorFromID_EmptyID(t *testing.T) {
	// Empty ID should return first color (fixed fallback)
	color := model.ColorFromID("")

	if color != model.FaviconColors[0] {
		t.Errorf("Empty ID should return first color %q, got %q", model.FaviconColors[0], color)
	}
}

func TestColorFromID_Distribution(t *testing.T) {
	// Different IDs should (mostly) produce different colors
	ids := []string{
		"p_abc123",
		"p_def456",
		"p_ghi789",
		"p_jkl012",
		"p_mno345",
	}

	colors := make(map[string]bool)
	for _, id := range ids {
		colors[model.ColorFromID(id)] = true
	}

	// With 5 IDs and 10 colors, we expect at least 2 different colors
	// (statistically very likely to have more)
	if len(colors) < 2 {
		t.Errorf("Expected some color variety, but got only %d unique colors for %d IDs", len(colors), len(ids))
	}
}

// Helper to verify TOML structure
func TestFileProjectStore_SavesValidTOML(t *testing.T) {
	store, dir, cleanup := setupTestProjectStore(t)
	defer cleanup()

	cfg := &model.ProjectConfig{
		ID:   "p_tomltest",
		Name: "TOML Test",
		Favicon: model.FaviconConfig{
			Background: "#3b82f6",
			IconType:   model.IconTypeLetter,
			Letter:     "T",
		},
	}

	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Read raw file and verify it's valid TOML
	path := filepath.Join(dir, ".kan", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var parsed map[string]any
	if err := toml.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Saved file is not valid TOML: %v", err)
	}

	// Verify key fields present
	if parsed["kan_schema"] != version.CurrentProjectSchema() {
		t.Errorf("kan_schema = %v, want %q", parsed["kan_schema"], version.CurrentProjectSchema())
	}
	if parsed["id"] != "p_tomltest" {
		t.Errorf("id = %v, want 'p_tomltest'", parsed["id"])
	}
}
