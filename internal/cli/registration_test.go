package cli

import (
	"testing"

	"github.com/amterp/kan/internal/store"
)

// Projects under the temp directory (where every test project lives, on
// macOS and Linux alike) are one-off copies, so discovering one must not
// register it.
func TestNewApp_TempDirProject_NotRegistered(t *testing.T) {
	openFixtureProject(t, "v13")

	if _, err := NewAppWithOptions(AppOptions{ReadOnly: true}); err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}

	cfg, err := store.NewGlobalStore().Load()
	if err != nil {
		t.Fatalf("load global config: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("temp project was registered: %v", cfg.Repos)
	}
}
