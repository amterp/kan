package store

import (
	"errors"
	"os"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
)

func TestReadOnlyCardStore_ReadsOverlayBeforeDisk(t *testing.T) {
	writable, dir, cleanup := setupTestCardStore(t)
	defer cleanup()
	paths := config.NewPaths(dir, "")

	if err := writable.Create("main", &model.Card{ID: "c1", Title: "on disk"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := writable.Create("main", &model.Card{ID: "c2", Title: "untouched"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	overlay := NewOverlay()
	overlay.Put(paths.CardPath("main", "c1"), []byte(`{"_v": 3, "id": "c1", "title": "in overlay"}`))
	ro := NewReadOnlyCardStore(paths, overlay)

	c1, err := ro.Get("main", "c1")
	if err != nil {
		t.Fatalf("Get c1: %v", err)
	}
	if c1.Title != "in overlay" {
		t.Errorf("c1 title = %q, want the overlay's", c1.Title)
	}
	c2, err := ro.Get("main", "c2")
	if err != nil {
		t.Fatalf("Get c2: %v", err)
	}
	if c2.Title != "untouched" {
		t.Errorf("c2 title = %q, want the file's", c2.Title)
	}
	cards, err := ro.List("main")
	if err != nil || len(cards) != 2 {
		t.Fatalf("List = %d cards, %v; want 2", len(cards), err)
	}
}

func TestReadOnlyStores_RefuseWrites(t *testing.T) {
	writable, dir, cleanup := setupTestCardStore(t)
	defer cleanup()
	paths := config.NewPaths(dir, "")

	card := &model.Card{ID: "c1", Title: "original"}
	if err := writable.Create("main", card); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cardPath := paths.CardPath("main", "c1")
	before, _ := os.ReadFile(cardPath)

	cards := NewReadOnlyCardStore(paths, nil)
	boards := NewReadOnlyBoardStore(paths, nil)
	project := NewReadOnlyProjectStore(paths, nil)

	card.Title = "changed"
	writes := map[string]error{
		"CardStore.Create":               cards.Create("main", &model.Card{ID: "c2"}),
		"CardStore.Update":               cards.Update("main", card),
		"CardStore.Delete":               cards.Delete("main", "c1"),
		"BoardStore.Create":              boards.Create(&model.BoardConfig{Name: "other"}),
		"BoardStore.Update":              boards.Update(&model.BoardConfig{Name: "main"}),
		"BoardStore.Delete":              boards.Delete("main"),
		"ProjectStore.Save":              project.Save(&model.ProjectConfig{Name: "x"}),
		"ProjectStore.EnsureInitialized": project.EnsureInitialized("x"),
	}
	for name, err := range writes {
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("%s = %v, want ErrReadOnly", name, err)
		}
	}

	after, _ := os.ReadFile(cardPath)
	if string(before) != string(after) {
		t.Error("card file changed through a read-only store")
	}
	if project.Exists() {
		t.Error("read-only project store created a project config")
	}
	if _, err := os.Stat(paths.BoardDir("other")); !os.IsNotExist(err) {
		t.Error("read-only board store created a board")
	}
}
