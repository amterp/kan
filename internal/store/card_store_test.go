package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
)

func setupTestCardStore(t *testing.T) (*FileCardStore, string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "kan-card-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create board structure
	cardsDir := filepath.Join(dir, ".kan", "boards", "main", "cards")
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create cards dir: %v", err)
	}

	paths := config.NewPaths(dir, "")
	store := NewCardStore(paths)

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return store, dir, cleanup
}

func TestFileCardStore_CreateAndGet(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	card := &model.Card{
		ID:              "test123",
		Alias:           "test-card",
		Title:           "Test Card",
		Creator:         "tester",
		CreatedAtMillis: 1704307200000,
		UpdatedAtMillis: 1704307200000,
	}

	// Create (store automatically stamps Version)
	if err := store.Create("main", card); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get by ID
	retrieved, err := store.Get("main", "test123")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != card.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, card.ID)
	}
	if retrieved.Title != card.Title {
		t.Errorf("Title mismatch: got %q, want %q", retrieved.Title, card.Title)
	}
	if retrieved.Version != 1 {
		t.Errorf("Version mismatch: got %d, want 1", retrieved.Version)
	}
}

func TestFileCardStore_GetNotFound(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	_, err := store.Get("main", "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent card")
	}

	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got: %v", err)
	}
}

func TestFileCardStore_Update(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	card := &model.Card{
		ID:              "test123",
		Alias:           "test-card",
		Title:           "Original Title",
		Creator:         "tester",
		CreatedAtMillis: 1704307200000,
		UpdatedAtMillis: 1704307200000,
	}

	if err := store.Create("main", card); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update
	card.Title = "Updated Title"
	card.Description = "New description"
	if err := store.Update("main", card); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify
	retrieved, err := store.Get("main", "test123")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Title != "Updated Title" {
		t.Errorf("Title not updated: got %q", retrieved.Title)
	}
	if retrieved.Description != "New description" {
		t.Errorf("Description not updated: got %q", retrieved.Description)
	}
}

func TestFileCardStore_Delete(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	card := &model.Card{
		ID:              "test123",
		Alias:           "test-card",
		Title:           "Test Card",
		Creator:         "tester",
		CreatedAtMillis: 1704307200000,
		UpdatedAtMillis: 1704307200000,
	}

	if err := store.Create("main", card); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete
	if err := store.Delete("main", "test123"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify gone
	_, err := store.Get("main", "test123")
	if !kanerr.IsNotFound(err) {
		t.Errorf("Card should be deleted, got err: %v", err)
	}
}

func TestFileCardStore_DeleteNotFound(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	err := store.Delete("main", "nonexistent")
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got: %v", err)
	}
}

func TestFileCardStore_List(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	// Create multiple cards
	cards := []*model.Card{
		{ID: "card1", Alias: "card-1", Title: "Card 1", Creator: "tester", CreatedAtMillis: 1, UpdatedAtMillis: 1},
		{ID: "card2", Alias: "card-2", Title: "Card 2", Creator: "tester", CreatedAtMillis: 2, UpdatedAtMillis: 2},
		{ID: "card3", Alias: "card-3", Title: "Card 3", Creator: "tester", CreatedAtMillis: 3, UpdatedAtMillis: 3},
	}

	for _, card := range cards {
		if err := store.Create("main", card); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List
	listed, err := store.List("main")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != 3 {
		t.Errorf("Expected 3 cards, got %d", len(listed))
	}
}

func TestFileCardStore_ListEmpty(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	listed, err := store.List("main")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should return empty slice, not nil
	if listed == nil {
		t.Error("Expected empty slice, got nil")
	}
	if len(listed) != 0 {
		t.Errorf("Expected 0 cards, got %d", len(listed))
	}
}

func TestFileCardStore_FindByAlias(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	card := &model.Card{
		ID:              "test123",
		Alias:           "my-unique-alias",
		Title:           "Test Card",
		Creator:         "tester",
		CreatedAtMillis: 1704307200000,
		UpdatedAtMillis: 1704307200000,
	}

	if err := store.Create("main", card); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Find by alias
	found, err := store.FindByAlias("main", "my-unique-alias")
	if err != nil {
		t.Fatalf("FindByAlias failed: %v", err)
	}

	if found.ID != "test123" {
		t.Errorf("Wrong card found: got ID %q", found.ID)
	}
}

func TestFileCardStore_FindByAliasNotFound(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	_, err := store.FindByAlias("main", "nonexistent-alias")
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got: %v", err)
	}
}

func TestFileCardStore_CustomFields(t *testing.T) {
	store, _, cleanup := setupTestCardStore(t)
	defer cleanup()

	card := &model.Card{
		ID:              "test123",
		Alias:           "test-card",
		Title:           "Test Card",
		Creator:         "tester",
		CreatedAtMillis: 1704307200000,
		UpdatedAtMillis: 1704307200000,
		CustomFields: map[string]any{
			"priority": "high",
			"estimate": 5,
		},
	}

	if err := store.Create("main", card); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := store.Get("main", "test123")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.CustomFields["priority"] != "high" {
		t.Errorf("CustomField priority not preserved: %v", retrieved.CustomFields)
	}
	// JSON numbers decode as float64
	if retrieved.CustomFields["estimate"] != float64(5) {
		t.Errorf("CustomField estimate not preserved: %v (type %T)", retrieved.CustomFields["estimate"], retrieved.CustomFields["estimate"])
	}
}
