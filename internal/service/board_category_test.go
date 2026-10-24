package service

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// setupFileBoardService returns a BoardService over real file stores in a
// temp project holding one default board, "main".
func setupFileBoardService(t *testing.T) (*BoardService, *store.FileBoardStore, *config.Paths) {
	t.Helper()
	paths := config.NewPaths(t.TempDir(), "")
	boardStore := store.NewBoardStore(paths)
	svc := NewBoardService(boardStore, store.NewCardStore(paths))
	if err := svc.Create("main"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	return svc, boardStore, paths
}

func setReadyOrder(t *testing.T, boardStore *store.FileBoardStore, order ...string) {
	t.Helper()
	cfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	cfg.Ready.Order = order
	if err := boardStore.Update(cfg); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
}

func readBoard(t *testing.T, boardStore *store.FileBoardStore) *model.BoardConfig {
	t.Helper()
	cfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	return cfg
}

func TestBoardService_Create_SetsDefaultCategories(t *testing.T) {
	_, boardStore, paths := setupFileBoardService(t)

	cfg := readBoard(t, boardStore)
	got := map[string]string{}
	for _, col := range cfg.Columns {
		got[col.Name] = col.Category
	}
	want := map[string]string{"backlog": "open", "next": "open", "in-progress": "active", "done": "done"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("categories = %v, want %v", got, want)
	}

	// A board with no [ready] settings must not gain an empty table on disk.
	data, err := os.ReadFile(paths.BoardConfigPath("main"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if strings.Contains(string(data), "[ready]") {
		t.Errorf("new board config should have no [ready] table:\n%s", data)
	}
}

func TestBoardService_UpdateColumnCategory(t *testing.T) {
	svc, boardStore, _ := setupFileBoardService(t)

	if err := svc.UpdateColumnCategory("main", "next", model.CategoryReview); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if got := readBoard(t, boardStore).GetColumn("next").Category; got != model.CategoryReview {
		t.Errorf("category = %q, want review", got)
	}

	if err := svc.UpdateColumnCategory("main", "next", ""); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if got := readBoard(t, boardStore).GetColumn("next").Category; got != "" {
		t.Errorf("category = %q, want cleared", got)
	}

	err := svc.UpdateColumnCategory("main", "next", "urgent")
	var verr *kanerr.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("invalid category: got %v, want a validation error", err)
	}
	if got := readBoard(t, boardStore).GetColumn("next").Category; got != "" {
		t.Errorf("invalid category was written: %q", got)
	}

	if err := svc.UpdateColumnCategory("main", "nope", model.CategoryOpen); !kanerr.IsNotFound(err) {
		t.Errorf("missing column: got %v, want not found", err)
	}
}

func TestBoardService_RenameColumn_KeepsReadyOrder(t *testing.T) {
	svc, boardStore, _ := setupFileBoardService(t)
	setReadyOrder(t, boardStore, "next", "backlog")

	if err := svc.RenameColumn("main", "next", "today"); err != nil {
		t.Fatalf("RenameColumn failed: %v", err)
	}
	if got := readBoard(t, boardStore).Ready.Order; !reflect.DeepEqual(got, []string{"today", "backlog"}) {
		t.Errorf("ready order = %v, want [today backlog]", got)
	}
}

func TestBoardService_DeleteColumn_DropsReadyOrderEntry(t *testing.T) {
	svc, boardStore, _ := setupFileBoardService(t)
	setReadyOrder(t, boardStore, "next", "backlog")

	if _, err := svc.DeleteColumn("main", "next"); err != nil {
		t.Fatalf("DeleteColumn failed: %v", err)
	}
	if got := readBoard(t, boardStore).Ready.Order; !reflect.DeepEqual(got, []string{"backlog"}) {
		t.Errorf("ready order = %v, want [backlog]", got)
	}
}
