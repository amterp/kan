package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

func saveRegistry(t *testing.T, repos map[string]model.RepoConfig) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := store.NewGlobalStore().Save(&model.GlobalConfig{Repos: repos}); err != nil {
		t.Fatalf("save global config: %v", err)
	}
	return config.GlobalConfigPath()
}

func TestRegistryHideUnhide(t *testing.T) {
	root := resolvedTempDir(t)
	makeBoardDir(t, filepath.Join(root, ".kan"))
	saveRegistry(t, map[string]model.RepoConfig{root: {}})

	runRegistrySetHidden(root, true)
	cfg, _ := store.NewGlobalStore().Load()
	if !cfg.Repos[root].Hidden {
		t.Fatal("hide did not set hidden")
	}

	runRegistrySetHidden(root, false)
	cfg, _ = store.NewGlobalStore().Load()
	if cfg.Repos[root].Hidden {
		t.Error("unhide did not clear hidden")
	}
}

func TestRegistryPrune_DryRunWritesNothing(t *testing.T) {
	gone := filepath.Join(resolvedTempDir(t), "deleted")
	path := saveRegistry(t, map[string]model.RepoConfig{gone: {}})
	before, _ := os.ReadFile(path)

	runRegistryPrune(true)
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("prune --dry-run changed the global config")
	}

	runRegistryPrune(false)
	cfg, _ := store.NewGlobalStore().Load()
	if len(cfg.Repos) != 0 {
		t.Errorf("prune left %v", cfg.Repos)
	}
}

func TestRegistryCommands_Parse(t *testing.T) {
	for _, args := range [][]string{
		{"registry", "list", "--hidden"},
		{"registry", "hide", "/x"},
		{"registry", "unhide", "/x"},
		{"registry", "remove", "/x"},
		{"registry", "prune", "--dry-run"},
	} {
		ctx := buildRootCmd()
		if err := ctx.RootCmd.ParseOrError(args); err != nil {
			t.Errorf("parse %v: %v", args, err)
		}
	}
}
