package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// fakeWorktrees reports whether the working directory is a linked worktree
// and where the main worktree is.
type fakeWorktrees struct {
	isWorktree bool
	mainRoot   string
}

func (f fakeWorktrees) IsWorktree() bool                     { return f.isWorktree }
func (f fakeWorktrees) GetMainWorktreeRoot() (string, error) { return f.mainRoot, nil }

// makeBoardDir creates a single board under dataDir and returns dataDir.
func makeBoardDir(t *testing.T, dataDir string) string {
	t.Helper()
	boardDir := filepath.Join(dataDir, "boards", "main")
	if err := os.MkdirAll(filepath.Join(boardDir, "cards"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "config.toml"), []byte("name = \"main\"\n"), 0644); err != nil {
		t.Fatalf("write board config: %v", err)
	}
	return dataDir
}

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	return dir
}

func TestMigrateTarget_UsesRegisteredDataLocation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := resolvedTempDir(t)
	makeBoardDir(t, filepath.Join(root, "kan-data"))

	cfg := &model.GlobalConfig{}
	cfg.SetRepoConfig(root, model.RepoConfig{DataLocation: "kan-data"})
	if err := store.NewGlobalStore().Save(cfg); err != nil {
		t.Fatalf("save global config: %v", err)
	}
	t.Chdir(root)

	paths, err := migrateTarget(fakeWorktrees{})
	if err != nil {
		t.Fatalf("migrateTarget: %v", err)
	}
	if want := filepath.Join(root, "kan-data"); paths.KanRoot() != want {
		t.Errorf("KanRoot = %q, want %q", paths.KanRoot(), want)
	}
}

func TestMigrateTarget_LinkedWorktreeMigratesMainBoard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mainRoot := resolvedTempDir(t)
	makeBoardDir(t, filepath.Join(mainRoot, ".kan"))
	worktreeRoot := resolvedTempDir(t)
	makeBoardDir(t, filepath.Join(worktreeRoot, ".kan"))
	t.Chdir(worktreeRoot)

	paths, err := migrateTarget(fakeWorktrees{isWorktree: true, mainRoot: mainRoot})
	if err != nil {
		t.Fatalf("migrateTarget: %v", err)
	}
	if paths.ProjectRoot() != mainRoot {
		t.Errorf("ProjectRoot = %q, want the main worktree %q", paths.ProjectRoot(), mainRoot)
	}
}

func TestMigrateTarget_NoProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(resolvedTempDir(t))

	if _, err := migrateTarget(fakeWorktrees{}); err == nil {
		t.Error("migrateTarget outside any project should fail")
	}
}
