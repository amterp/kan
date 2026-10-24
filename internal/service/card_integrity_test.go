package service

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

func setupCardIntegrityTest(t *testing.T) (*CardService, *config.Paths) {
	t.Helper()
	paths := config.NewPaths(t.TempDir(), "")
	cards := store.NewCardStore(paths)
	boards := store.NewBoardStore(paths)
	if err := boards.Create(&model.BoardConfig{
		Name: "main", DefaultColumn: "backlog",
		Columns: []model.Column{{Name: "backlog"}, {Name: "done"}},
	}); err != nil {
		t.Fatal(err)
	}
	svc := NewCardService(cards, boards, NewAliasService(cards))
	nextID := 0
	svc.generateCardID = func() string {
		nextID++
		return fmt.Sprintf("card-%d", nextID)
	}
	return svc, paths
}

func readCardBytes(t *testing.T, paths *config.Paths, cardID string) []byte {
	t.Helper()
	data, err := os.ReadFile(paths.CardPath("main", cardID))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCardService_AddIDCollisions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ids       []string
		wantError bool
	}{
		{name: "retry succeeds", ids: []string{"existing", "fresh"}},
		{name: "fifth attempt succeeds", ids: []string{"existing", "existing", "existing", "existing", "fresh"}},
		{name: "five collisions", ids: []string{"existing", "existing", "existing", "existing", "existing"}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, paths := setupCardIntegrityTest(t)
			existing := &model.Card{ID: "existing", Title: "Original", Alias: "original", Column: "backlog", Position: "a0"}
			if err := svc.cardStore.Create("main", existing); err != nil {
				t.Fatal(err)
			}
			before := readCardBytes(t, paths, existing.ID)
			remaining := append([]string(nil), tc.ids...)
			svc.generateCardID = func() string {
				if len(remaining) == 0 {
					t.Fatal("generated an ID after the allowed attempts")
				}
				value := remaining[0]
				remaining = remaining[1:]
				return value
			}
			card, _, err := svc.Add(AddCardInput{BoardName: "main", Title: "New card"})
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "5 ID collisions") {
					t.Fatalf("expected collision exhaustion, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				stored, err := svc.Get("main", "fresh")
				if err != nil || stored.Title != "New card" || card.ID != stored.ID {
					t.Fatalf("new card was not persisted with its successful ID: card=%+v stored=%+v err=%v", card, stored, err)
				}
			}
			if len(remaining) != 0 {
				t.Errorf("creation stopped with %d expected attempts left", len(remaining))
			}
			if !bytes.Equal(before, readCardBytes(t, paths, existing.ID)) {
				t.Fatal("collision changed the existing card")
			}
		})
	}
}

func TestCardService_AddDoesNotRetryOtherWriteErrors(t *testing.T) {
	svc, _ := setupCardIntegrityTest(t)
	originalStore := svc.cardStore
	svc.cardStore = &failingCreateCardStore{CardStore: originalStore}
	ids := []string{"first"}
	svc.generateCardID = func() string {
		if len(ids) == 0 {
			t.Fatal("retried a non-collision error")
		}
		value := ids[0]
		ids = ids[1:]
		return value
	}
	if _, _, err := svc.Add(AddCardInput{BoardName: "main", Title: "New"}); err == nil {
		t.Fatal("expected write failure")
	}
	cards, err := originalStore.List("main")
	if err != nil || len(cards) != 0 {
		t.Fatalf("failed creation left cards: %v, %v", cards, err)
	}
}

type failingCreateCardStore struct {
	store.CardStore
}

func (s *failingCreateCardStore) Create(string, *model.Card) error {
	return fmt.Errorf("write denied: %w", fs.ErrPermission)
}

func TestCardService_RestorePreservesExistingCard(t *testing.T) {
	svc, paths := setupCardIntegrityTest(t)
	card, _, err := svc.Add(AddCardInput{BoardName: "main", Title: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	before := readCardBytes(t, paths, card.ID)
	snapshot := *card
	snapshot.Title = "Restored snapshot"
	if err := svc.Restore("main", &snapshot, "backlog", -1); err == nil {
		t.Fatal("expected restore to refuse an existing ID")
	}
	if !bytes.Equal(before, readCardBytes(t, paths, card.ID)) {
		t.Fatal("restore replaced the existing card")
	}
	if err := svc.Delete("main", card.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Restore("main", &snapshot, "backlog", -1); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.Get("main", card.ID)
	if err != nil || stored.ID != card.ID || stored.Title != snapshot.Title {
		t.Fatalf("restore did not preserve the snapshot: %+v, %v", stored, err)
	}
}
