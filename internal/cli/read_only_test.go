package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

// migrationFixtures is absolute because tests change the working directory.
var migrationFixtures, _ = filepath.Abs(filepath.Join("..", "service", "testdata", "migrations"))

// openFixtureProject copies a migration fixture from the service package
// into a fresh project directory, isolates HOME, and makes the project the
// working directory. Returns the project root.
func openFixtureProject(t *testing.T, fixture string) string {
	t.Helper()
	root := copyFixture(t, fixture)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	return root
}

// copyFixture copies a migration fixture from the service package into a
// fresh directory and returns it.
func copyFixture(t *testing.T, fixture string) string {
	t.Helper()
	src := filepath.Join(migrationFixtures, fixture)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		dst := filepath.Join(root, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0644)
	})
	if err != nil {
		t.Fatalf("copy fixture %s: %v", fixture, err)
	}
	return root
}

// snapshotProject maps every file under root to its contents.
func snapshotProject(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		files[path] = data
		return err
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return files
}

func assertProjectUnchanged(t *testing.T, before, after map[string][]byte) {
	t.Helper()
	for path, data := range after {
		old, existed := before[path]
		if !existed {
			t.Errorf("read command created %s", path)
		} else if !bytes.Equal(old, data) {
			t.Errorf("read command rewrote %s", path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			t.Errorf("read command removed %s", path)
		}
	}
}

func TestReadOnlyApp_OlderProject_ReadsMigratedDataWithoutWriting(t *testing.T) {
	// board/2 with card_ids in the board config, a card/1 card with no
	// column, and a project/1 config.
	root := openFixtureProject(t, "v2")
	before := snapshotProject(t, root)

	app, err := NewAppWithOptions(AppOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}
	if err := app.RequireKan(); err != nil {
		t.Fatalf("RequireKan: %v", err)
	}

	board, err := app.BoardStore.Get("main")
	if err != nil {
		t.Fatalf("BoardStore.Get: %v", err)
	}
	if board.KanSchema != version.CurrentBoardSchema() {
		t.Errorf("board schema = %s, want it migrated in memory to %s", board.KanSchema, version.CurrentBoardSchema())
	}
	card, err := app.CardStore.Get("main", "card-abc")
	if err != nil {
		t.Fatalf("CardStore.Get: %v", err)
	}
	if card.Column == "" || card.Position == "" {
		t.Errorf("card should carry column and position from the board's card_ids, got %q / %q", card.Column, card.Position)
	}
	if _, err := app.ProjectStore.Load(); err != nil {
		t.Errorf("ProjectStore.Load on a project/1 config: %v", err)
	}
	if err := app.CardStore.Update("main", card); !errors.Is(err, store.ErrReadOnly) {
		t.Errorf("CardStore.Update = %v, want ErrReadOnly", err)
	}

	assertProjectUnchanged(t, before, snapshotProject(t, root))
}

func TestReadOnlyApp_MissingProjectConfig_NotCreated(t *testing.T) {
	root := openFixtureProject(t, "project_missing")
	before := snapshotProject(t, root)

	app, err := NewAppWithOptions(AppOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}
	if _, err := app.CardStore.List("main"); err != nil {
		t.Fatalf("List: %v", err)
	}

	assertProjectUnchanged(t, before, snapshotProject(t, root))
}

func TestReadOnlyApp_FutureProjectConfig_Fails(t *testing.T) {
	openFixtureProject(t, "project_future")

	_, err := NewAppWithOptions(AppOptions{ReadOnly: true})
	var schemaErr *version.SchemaVersionError
	if !errors.As(err, &schemaErr) {
		t.Fatalf("NewAppWithOptions = %v, want SchemaVersionError", err)
	}
}

func TestWriteApp_OlderProject_Migrates(t *testing.T) {
	root := openFixtureProject(t, "v2")

	if _, err := NewAppWithOptions(AppOptions{}); err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".kan", "boards", "main", "config.toml"))
	if err != nil {
		t.Fatalf("read board config: %v", err)
	}
	if !bytes.Contains(data, []byte(version.CurrentBoardSchema())) {
		t.Errorf("a write command should migrate the board on disk, got:\n%s", data)
	}
}

func TestCompletion_OlderProject_ListsCardsWithoutWriting(t *testing.T) {
	root := openFixtureProject(t, "v2")
	before := snapshotProject(t, root)
	compCtx = completionCtx{}
	t.Cleanup(func() { compCtx = completionCtx{} })

	cards, _ := completeCards("")
	if len(cards) == 0 {
		t.Error("completion on an older project should still offer its cards")
	}

	assertProjectUnchanged(t, before, snapshotProject(t, root))
}

// A syntactically broken project config costs a warning, as it did before
// the project config joined the migration plan; commands still run.
func TestBrokenProjectConfig_CommandsStillRun(t *testing.T) {
	root := openFixtureProject(t, "v15")
	if err := os.WriteFile(filepath.Join(root, ".kan", "config.toml"), []byte("name = [unclosed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, readOnly := range []bool{true, false} {
		app, err := NewAppWithOptions(AppOptions{ReadOnly: readOnly})
		if err != nil {
			t.Fatalf("ReadOnly=%v: NewAppWithOptions: %v", readOnly, err)
		}
		if _, _, err := app.ResolveBoard("", false, forRead); err != nil {
			t.Errorf("ReadOnly=%v: resolve board: %v", readOnly, err)
		}
	}
}
