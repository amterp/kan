package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
)

func setupTestBoardStore(t *testing.T) (*FileBoardStore, string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "kan-board-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create .kan/boards structure
	boardsDir := filepath.Join(dir, ".kan", "boards")
	if err := os.MkdirAll(boardsDir, 0755); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create boards dir: %v", err)
	}

	paths := config.NewPaths(dir, "")
	store := NewBoardStore(paths)

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return store, dir, cleanup
}

func TestFileBoardStore_CreateAndGet(t *testing.T) {
	store, _, cleanup := setupTestBoardStore(t)
	defer cleanup()

	cfg := &model.BoardConfig{
		ID:            "board123",
		Name:          "main",
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
	}

	// Create
	if err := store.Create(cfg); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get
	retrieved, err := store.Get("main")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != cfg.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, cfg.ID)
	}
	if retrieved.Name != cfg.Name {
		t.Errorf("Name mismatch: got %q, want %q", retrieved.Name, cfg.Name)
	}
	if len(retrieved.Columns) != len(cfg.Columns) {
		t.Errorf("Columns count mismatch: got %d, want %d", len(retrieved.Columns), len(cfg.Columns))
	}
}

func TestFileBoardStore_CreateDuplicate(t *testing.T) {
	store, _, cleanup := setupTestBoardStore(t)
	defer cleanup()

	cfg := &model.BoardConfig{
		ID:            "board123",
		Name:          "main",
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
	}

	if err := store.Create(cfg); err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	// Try to create again
	err := store.Create(cfg)
	if err == nil {
		t.Fatal("Expected error for duplicate board")
	}

	if !kanerr.IsAlreadyExists(err) {
		t.Errorf("Expected AlreadyExists error, got: %v", err)
	}
}

func TestFileBoardStore_GetNotFound(t *testing.T) {
	store, _, cleanup := setupTestBoardStore(t)
	defer cleanup()

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent board")
	}

	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got: %v", err)
	}
}

func TestFileBoardStore_Update(t *testing.T) {
	store, _, cleanup := setupTestBoardStore(t)
	defer cleanup()

	cfg := &model.BoardConfig{
		ID:            "board123",
		Name:          "main",
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
	}

	if err := store.Create(cfg); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update
	cfg.DefaultColumn = "next"
	cfg.CustomFields = map[string]model.CustomFieldSchema{
		"priority": {Type: "enum", Options: []model.CustomFieldOption{
			{Value: "low"}, {Value: "high"},
		}},
	}
	if err := store.Update(cfg); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify
	retrieved, err := store.Get("main")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.DefaultColumn != "next" {
		t.Errorf("DefaultColumn not updated: got %q", retrieved.DefaultColumn)
	}
	if len(retrieved.CustomFields) != 1 {
		t.Errorf("CustomFields not updated: got %d", len(retrieved.CustomFields))
	}
}

func TestFileBoardStore_List(t *testing.T) {
	store, _, cleanup := setupTestBoardStore(t)
	defer cleanup()

	// Create multiple boards
	boards := []string{"main", "features", "bugs"}
	for _, name := range boards {
		cfg := &model.BoardConfig{
			ID:            name + "-id",
			Name:          name,
			Columns:       model.DefaultColumns(),
			DefaultColumn: "backlog",
		}
		if err := store.Create(cfg); err != nil {
			t.Fatalf("Create %s failed: %v", name, err)
		}
	}

	// List
	listed, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != 3 {
		t.Errorf("Expected 3 boards, got %d", len(listed))
	}
}

func TestFileBoardStore_ListEmpty(t *testing.T) {
	store, _, cleanup := setupTestBoardStore(t)
	defer cleanup()

	listed, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should return empty slice, not nil
	if listed == nil {
		t.Error("Expected empty slice, got nil")
	}
	if len(listed) != 0 {
		t.Errorf("Expected 0 boards, got %d", len(listed))
	}
}

func TestFileBoardStore_Exists(t *testing.T) {
	store, _, cleanup := setupTestBoardStore(t)
	defer cleanup()

	cfg := &model.BoardConfig{
		ID:            "board123",
		Name:          "main",
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
	}

	// Before create
	if store.Exists("main") {
		t.Error("Board should not exist before creation")
	}

	if err := store.Create(cfg); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// After create
	if !store.Exists("main") {
		t.Error("Board should exist after creation")
	}

	// Nonexistent
	if store.Exists("nonexistent") {
		t.Error("Nonexistent board should not exist")
	}
}

func TestFileBoardStore_WithCustomFieldsAndCardDisplay(t *testing.T) {
	store, _, cleanup := setupTestBoardStore(t)
	defer cleanup()

	cfg := &model.BoardConfig{
		ID:            "board123",
		Name:          "main",
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
		CustomFields: map[string]model.CustomFieldSchema{
			"type": {Type: "enum", Options: []model.CustomFieldOption{
				{Value: "feature", Color: "#16a34a"},
				{Value: "bug", Color: "#dc2626"},
			}},
			"labels": {Type: "tags", Options: []model.CustomFieldOption{
				{Value: "blocked", Color: "#dc2626"},
				{Value: "needs-review", Color: "#f59e0b"},
			}},
			"assignee": {Type: "string"},
		},
		CardDisplay: model.CardDisplayConfig{
			TypeIndicator: "type",
			Badges:        []string{"labels"},
		},
	}

	if err := store.Create(cfg); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := store.Get("main")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(retrieved.CustomFields) != 3 {
		t.Errorf("CustomFields not preserved: got %d", len(retrieved.CustomFields))
	}
	if retrieved.CustomFields["type"].Type != "enum" {
		t.Errorf("CustomField type not preserved: got %q", retrieved.CustomFields["type"].Type)
	}
	if retrieved.CustomFields["labels"].Type != "tags" {
		t.Errorf("CustomField labels type not preserved: got %q", retrieved.CustomFields["labels"].Type)
	}
	if retrieved.CardDisplay.TypeIndicator != "type" {
		t.Errorf("CardDisplay.TypeIndicator not preserved: got %q", retrieved.CardDisplay.TypeIndicator)
	}
	if len(retrieved.CardDisplay.Badges) != 1 || retrieved.CardDisplay.Badges[0] != "labels" {
		t.Errorf("CardDisplay.Badges not preserved: got %v", retrieved.CardDisplay.Badges)
	}
}
