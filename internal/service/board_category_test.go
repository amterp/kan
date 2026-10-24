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

// setupCustomColumnsBoard returns a BoardService over real file stores with
// one board, "main", whose columns are given by name; done is pre-categorized.
func setupCustomColumnsBoard(t *testing.T, names ...string) (*BoardService, *store.FileBoardStore) {
	t.Helper()
	paths := config.NewPaths(t.TempDir(), "")
	boardStore := store.NewBoardStore(paths)
	cfg := &model.BoardConfig{ID: "b_test", Name: "main", DefaultColumn: names[0]}
	for _, name := range names {
		col := model.Column{Name: name, Color: "#6b7280"}
		if name == "done" {
			col.Category = model.CategoryDropped
		}
		cfg.Columns = append(cfg.Columns, col)
	}
	if err := boardStore.Create(cfg); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	return NewBoardService(boardStore, store.NewCardStore(paths)), boardStore
}

func categoriesOf(cfg *model.BoardConfig) map[string]string {
	got := map[string]string{}
	for _, col := range cfg.Columns {
		got[col.Name] = col.Category
	}
	return got
}

func TestBoardService_CategorizeColumns_SuggestsWithoutWriting(t *testing.T) {
	svc, boardStore := setupCustomColumnsBoard(t, "inbox", "ideas", "backlog-hi", "doing", "p0", "wont-do", "done")

	got, err := svc.CategorizeColumns("main", false)
	if err != nil {
		t.Fatalf("CategorizeColumns failed: %v", err)
	}
	want := []CategorySuggestion{
		{Column: "inbox", Category: "inbox"},
		{Column: "ideas", Category: "idea"},
		{Column: "backlog-hi", Category: "open"},
		{Column: "doing", Category: "active"},
		{Column: "p0", Category: ""},
		{Column: "wont-do", Category: "dropped"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("suggestions = %+v, want %+v", got, want)
	}

	for name, category := range categoriesOf(readBoard(t, boardStore)) {
		if name != "done" && category != "" {
			t.Errorf("column %q was written (%q) without apply", name, category)
		}
	}
}

func TestBoardService_CategorizeColumns_Apply(t *testing.T) {
	svc, boardStore := setupCustomColumnsBoard(t, "backlog", "review", "p0", "done")

	if _, err := svc.CategorizeColumns("main", true); err != nil {
		t.Fatalf("CategorizeColumns failed: %v", err)
	}
	want := map[string]string{"backlog": "open", "review": "review", "p0": "", "done": "dropped"}
	if got := categoriesOf(readBoard(t, boardStore)); !reflect.DeepEqual(got, want) {
		t.Errorf("categories = %v, want %v (an existing category must survive)", got, want)
	}

	again, err := svc.CategorizeColumns("main", true)
	if err != nil {
		t.Fatalf("second CategorizeColumns failed: %v", err)
	}
	if !reflect.DeepEqual(again, []CategorySuggestion{{Column: "p0"}}) {
		t.Errorf("second run = %+v, want only the unmatched column", again)
	}
}

func TestBoardService_CategorizeColumns_IgnoresCase(t *testing.T) {
	svc, _ := setupCustomColumnsBoard(t, "Backlog", "In-Progress", "Done", "Backlogs")

	got, err := svc.CategorizeColumns("main", false)
	if err != nil {
		t.Fatalf("CategorizeColumns failed: %v", err)
	}
	want := []CategorySuggestion{
		{Column: "Backlog", Category: "open"},
		{Column: "In-Progress", Category: "active"},
		{Column: "Done", Category: "done"},
		{Column: "Backlogs", Category: ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("suggestions = %+v, want %+v", got, want)
	}
}
