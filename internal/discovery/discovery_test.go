package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
)

func TestDiscoverProjectFrom_GlobalConfigEntry(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	kanDir := filepath.Join(projectDir, ".kan", "boards")
	if err := os.MkdirAll(kanDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Global config knows about this project
	globalCfg := &model.GlobalConfig{
		Repos: map[string]model.RepoConfig{
			projectDir: {DataLocation: ""},
		},
	}

	result, err := DiscoverProjectFrom(projectDir, globalCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ProjectRoot != projectDir {
		t.Errorf("expected ProjectRoot %q, got %q", projectDir, result.ProjectRoot)
	}
	if !result.WasRegistered {
		t.Error("expected WasRegistered to be true")
	}
}

func TestDiscoverProjectFrom_GlobalConfigEntry_CustomLocation(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	customKanDir := filepath.Join(projectDir, "custom", "kan", "boards")
	if err := os.MkdirAll(customKanDir, 0755); err != nil {
		t.Fatal(err)
	}

	globalCfg := &model.GlobalConfig{
		Repos: map[string]model.RepoConfig{
			projectDir: {DataLocation: "custom/kan"},
		},
	}

	result, err := DiscoverProjectFrom(projectDir, globalCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.DataLocation != "custom/kan" {
		t.Errorf("expected DataLocation 'custom/kan', got %q", result.DataLocation)
	}
	if !result.WasRegistered {
		t.Error("expected WasRegistered to be true")
	}
}

func TestDiscoverProjectFrom_SelfDiscoverable(t *testing.T) {
	// .kan/ exists but no global config entry
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	kanDir := filepath.Join(projectDir, ".kan", "boards")
	if err := os.MkdirAll(kanDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Empty global config - no entry for this project
	globalCfg := &model.GlobalConfig{}

	result, err := DiscoverProjectFrom(projectDir, globalCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ProjectRoot != projectDir {
		t.Errorf("expected ProjectRoot %q, got %q", projectDir, result.ProjectRoot)
	}
	if result.WasRegistered {
		t.Error("expected WasRegistered to be false for self-discovered project")
	}
	if result.DataLocation != "" {
		t.Errorf("expected empty DataLocation for default .kan/, got %q", result.DataLocation)
	}
}

func TestDiscoverProjectFrom_WalksUpDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	kanDir := filepath.Join(projectDir, ".kan", "boards")
	deepDir := filepath.Join(projectDir, "src", "deep", "nested")

	if err := os.MkdirAll(kanDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	globalCfg := &model.GlobalConfig{}

	// Start from deep nested directory
	result, err := DiscoverProjectFrom(deepDir, globalCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ProjectRoot != projectDir {
		t.Errorf("expected ProjectRoot %q, got %q", projectDir, result.ProjectRoot)
	}
}

func TestDiscoverProjectFrom_GlobalConfigTakesPrecedence(t *testing.T) {
	// Both global config entry AND .kan/ exist - global config should win
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	kanDir := filepath.Join(projectDir, ".kan", "boards")
	if err := os.MkdirAll(kanDir, 0755); err != nil {
		t.Fatal(err)
	}

	globalCfg := &model.GlobalConfig{
		Repos: map[string]model.RepoConfig{
			projectDir: {DataLocation: ""}, // Explicitly registered
		},
	}

	result, err := DiscoverProjectFrom(projectDir, globalCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// Should be registered because global config entry exists
	if !result.WasRegistered {
		t.Error("expected WasRegistered to be true when global config entry exists")
	}
}

func TestDiscoverProjectFrom_GlobalConfigMissingData(t *testing.T) {
	// Global config says project exists, but data is missing
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Note: NOT creating .kan/boards/

	globalCfg := &model.GlobalConfig{
		Repos: map[string]model.RepoConfig{
			projectDir: {DataLocation: ""},
		},
	}

	_, err := DiscoverProjectFrom(projectDir, globalCfg)
	if err == nil {
		t.Fatal("expected error when global config references missing data")
	}
}

func TestDiscoverProjectFrom_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	globalCfg := &model.GlobalConfig{}

	result, err := DiscoverProjectFrom(emptyDir, globalCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for uninitialized directory, got %+v", result)
	}
}

func TestDiscoverProjectFrom_NilGlobalConfig(t *testing.T) {
	// Should still work with nil global config (falls back to .kan/ discovery)
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	kanDir := filepath.Join(projectDir, config.DefaultKanDir, config.BoardsDir)
	if err := os.MkdirAll(kanDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := DiscoverProjectFrom(projectDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ProjectRoot != projectDir {
		t.Errorf("expected ProjectRoot %q, got %q", projectDir, result.ProjectRoot)
	}
}

func TestDiscoverProjectFrom_NestedProjects(t *testing.T) {
	// Inner project should be found first when starting from within it
	tmpDir := t.TempDir()
	outerProject := filepath.Join(tmpDir, "outer")
	innerProject := filepath.Join(outerProject, "inner")
	outerKan := filepath.Join(outerProject, ".kan", "boards")
	innerKan := filepath.Join(innerProject, ".kan", "boards")

	if err := os.MkdirAll(outerKan, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(innerKan, 0755); err != nil {
		t.Fatal(err)
	}

	globalCfg := &model.GlobalConfig{}

	// Starting from inner project should find inner, not outer
	result, err := DiscoverProjectFrom(innerProject, globalCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ProjectRoot != innerProject {
		t.Errorf("expected inner project %q, got %q", innerProject, result.ProjectRoot)
	}
}
