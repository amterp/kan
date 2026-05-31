package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

// writeProjectBoard creates a temp project with a single board at the current
// schema and returns the project root.
func writeProjectBoard(t *testing.T, board string) string {
	t.Helper()
	root := t.TempDir()
	boardDir := filepath.Join(root, ".kan", "boards", board)
	if err := os.MkdirAll(filepath.Join(boardDir, "cards"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := map[string]any{
		"kan_schema":     version.CurrentBoardSchema(),
		"name":           board,
		"id":             board + "-id",
		"default_column": "Backlog",
		"columns": []map[string]any{
			{"name": "Backlog", "color": "#6b7280"},
			{"name": "Done", "color": "#10b981"},
		},
	}
	f, err := os.Create(filepath.Join(boardDir, "config.toml"))
	if err != nil {
		t.Fatalf("create board config: %v", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		t.Fatalf("encode board config: %v", err)
	}
	return root
}

// designateGlobalBoard isolates HOME and writes a global config designating the
// given project/board as the global board.
func designateGlobalBoard(t *testing.T, projectRoot, board string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg := &model.GlobalConfig{}
	cfg.RegisterProject(filepath.Base(projectRoot), projectRoot)
	cfg.SetRepoConfig(projectRoot, model.RepoConfig{})
	if board != "" {
		cfg.SetGlobalBoard(projectRoot, board)
	}
	if err := store.NewGlobalStore().Save(cfg); err != nil {
		t.Fatalf("save global config: %v", err)
	}
}

func TestNewAppWithOptions_GlobalBoard(t *testing.T) {
	root := writeProjectBoard(t, "inbox")
	designateGlobalBoard(t, root, "inbox")

	app, err := NewAppWithOptions(AppOptions{Interactive: false, UseGlobalBoard: true})
	if err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}
	if !app.UsingGlobalBoard {
		t.Error("expected UsingGlobalBoard to be true")
	}
	if app.ProjectRoot != root {
		t.Errorf("ProjectRoot = %q, want %q", app.ProjectRoot, root)
	}
	if err := app.RequireKan(); err != nil {
		t.Fatalf("RequireKan: %v", err)
	}

	board, err := app.BoardResolver.Resolve("", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if board != "inbox" {
		t.Errorf("resolved board = %q, want inbox", board)
	}
}

func TestNewAppWithOptions_GlobalBoard_NotSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no global config at all

	_, err := NewAppWithOptions(AppOptions{UseGlobalBoard: true})
	var want *kanerr.NoGlobalBoardError
	if !errors.As(err, &want) {
		t.Fatalf("expected NoGlobalBoardError, got %v", err)
	}
}

// addBoardDir creates an additional board (current schema) under an existing
// project root.
func addBoardDir(t *testing.T, root, board string) {
	t.Helper()
	boardDir := filepath.Join(root, ".kan", "boards", board)
	if err := os.MkdirAll(filepath.Join(boardDir, "cards"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := map[string]any{
		"kan_schema":     version.CurrentBoardSchema(),
		"name":           board,
		"id":             board + "-id",
		"default_column": "Backlog",
		"columns":        []map[string]any{{"name": "Backlog", "color": "#6b7280"}},
	}
	f, err := os.Create(filepath.Join(boardDir, "config.toml"))
	if err != nil {
		t.Fatalf("create board config: %v", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		t.Fatalf("encode board config: %v", err)
	}
}

// TestResolveCardWithBoard_GlobalScopedToBoard guards the "-g means this board"
// contract: a card living on a non-designated board in the same global project
// must NOT resolve via cross-board search under -g.
func TestResolveCardWithBoard_GlobalScopedToBoard(t *testing.T) {
	root := writeProjectBoard(t, "inbox")
	addBoardDir(t, root, "archive")
	designateGlobalBoard(t, root, "inbox")

	// Put a card on "archive" (not the designated board).
	cardStore := store.NewCardStore(config.NewPaths(root, ""))
	if err := cardStore.Create("archive", &model.Card{
		Version: version.CurrentCardVersion,
		ID:      "c_archived",
		Alias:   "archived-task",
		Title:   "Archived task",
		Column:  "Backlog",
		Creator: "tester",
	}); err != nil {
		t.Fatalf("seed archive card: %v", err)
	}

	app, err := NewAppWithOptions(AppOptions{UseGlobalBoard: true})
	if err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}

	// The archive card must not be reachable under -g - it's on a different board.
	if _, err := app.ResolveCardWithBoard("", "archived-task", false); !kanerr.IsNotFound(err) {
		t.Errorf("expected not-found for a card outside the global board, got %v", err)
	}

	// An explicit -b still reaches it (power-user override).
	res, err := app.ResolveCardWithBoard("archive", "archived-task", false)
	if err != nil {
		t.Fatalf("explicit -b archive should resolve: %v", err)
	}
	if res.BoardName != "archive" {
		t.Errorf("BoardName = %q, want archive", res.BoardName)
	}
}

func TestNewAppWithOptions_GlobalBoard_Stale(t *testing.T) {
	root := writeProjectBoard(t, "inbox")
	// Designate a board that doesn't exist in the project.
	designateGlobalBoard(t, root, "ghost")

	_, err := NewAppWithOptions(AppOptions{UseGlobalBoard: true})
	var want *kanerr.StaleGlobalBoardError
	if !errors.As(err, &want) {
		t.Fatalf("expected StaleGlobalBoardError, got %v", err)
	}
}
