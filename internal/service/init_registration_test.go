package service

import (
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/store"
)

func initIn(t *testing.T, location string) (string, *store.FileGlobalStore) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	t.Chdir(root)
	gs := store.NewGlobalStore()
	if _, err := NewInitService(gs).Initialize(location, "", nil, "p", false); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return root, gs
}

func TestInitialize_TempDir_RegistersNothing(t *testing.T) {
	_, gs := initIn(t, "")
	cfg, err := gs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("kan init in a temp dir registered %v", cfg.Repos)
	}
}

func TestInitialize_TempDirCustomLocation_RegistersHidden(t *testing.T) {
	root, gs := initIn(t, "kan-data")
	cfg, err := gs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	repo := cfg.GetRepoConfig(root)
	if repo == nil || !repo.Hidden || repo.DataLocation != "kan-data" {
		t.Errorf("want a hidden entry with its data location, got %+v (repos %v)", repo, cfg.Repos)
	}
}
