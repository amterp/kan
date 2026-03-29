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

// --- WorktreeResolver mock ---

type mockWorktreeResolver struct {
	isWorktree bool
	mainRoot   string
	err        error
}

func (m *mockWorktreeResolver) IsWorktree() bool {
	return m.isWorktree
}

func (m *mockWorktreeResolver) GetMainWorktreeRoot() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.mainRoot, nil
}

func TestResolveWorktree_NotInWorktree(t *testing.T) {
	result := &Result{ProjectRoot: "/some/path", DataLocation: ""}
	resolver := &mockWorktreeResolver{isWorktree: false}

	resolved, err := ResolveWorktree(result, resolver, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.ResolvedFromWorktree {
		t.Error("should not resolve from worktree when not in a worktree")
	}
	if resolved.ProjectRoot != "/some/path" {
		t.Errorf("expected original project root, got %q", resolved.ProjectRoot)
	}
}

func TestResolveWorktree_RedirectsToMain(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	wtDir := filepath.Join(tmpDir, "worktree")

	// Create .kan in main
	if err := os.MkdirAll(filepath.Join(mainDir, ".kan", "boards"), 0755); err != nil {
		t.Fatal(err)
	}
	// Create .kan in worktree (as git would check out)
	if err := os.MkdirAll(filepath.Join(wtDir, ".kan", "boards"), 0755); err != nil {
		t.Fatal(err)
	}

	worktreeResult := &Result{ProjectRoot: wtDir, DataLocation: ""}
	resolver := &mockWorktreeResolver{isWorktree: true, mainRoot: mainDir}

	resolved, err := ResolveWorktree(worktreeResult, resolver, &model.GlobalConfig{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved.ResolvedFromWorktree {
		t.Error("expected ResolvedFromWorktree to be true")
	}
	if resolved.ProjectRoot != mainDir {
		t.Errorf("expected main dir %q, got %q", mainDir, resolved.ProjectRoot)
	}
	if resolved.OriginalWorktreeRoot != wtDir {
		t.Errorf("expected original worktree root %q, got %q", wtDir, resolved.OriginalWorktreeRoot)
	}
}

func TestResolveWorktree_IndependentOptOut(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	wtDir := filepath.Join(tmpDir, "worktree")

	// Create .kan in both
	if err := os.MkdirAll(filepath.Join(mainDir, ".kan", "boards"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wtDir, ".kan", "boards"), 0755); err != nil {
		t.Fatal(err)
	}

	worktreeResult := &Result{ProjectRoot: wtDir, DataLocation: ""}
	resolver := &mockWorktreeResolver{isWorktree: true, mainRoot: mainDir}

	// Independence check returns true - should NOT redirect
	isIndependent := func(projectRoot, dataLocation string) bool { return true }

	resolved, err := ResolveWorktree(worktreeResult, resolver, &model.GlobalConfig{}, isIndependent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.ResolvedFromWorktree {
		t.Error("should not redirect when project is independent")
	}
	if resolved.ProjectRoot != wtDir {
		t.Errorf("expected worktree root %q, got %q", wtDir, resolved.ProjectRoot)
	}
}

func TestResolveWorktree_MainHasNoProject(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	wtDir := filepath.Join(tmpDir, "worktree")

	// Main has NO .kan
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Worktree has .kan
	if err := os.MkdirAll(filepath.Join(wtDir, ".kan", "boards"), 0755); err != nil {
		t.Fatal(err)
	}

	worktreeResult := &Result{ProjectRoot: wtDir, DataLocation: ""}
	resolver := &mockWorktreeResolver{isWorktree: true, mainRoot: mainDir}

	resolved, err := ResolveWorktree(worktreeResult, resolver, &model.GlobalConfig{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to worktree since main has no project
	if resolved.ResolvedFromWorktree {
		t.Error("should not redirect when main has no project")
	}
	if resolved.ProjectRoot != wtDir {
		t.Errorf("expected worktree root %q, got %q", wtDir, resolved.ProjectRoot)
	}
}

func TestResolveWorktree_NilResult(t *testing.T) {
	resolver := &mockWorktreeResolver{isWorktree: true, mainRoot: "/some/main"}

	resolved, err := ResolveWorktree(nil, resolver, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != nil {
		t.Error("expected nil result for nil input")
	}
}

func TestResolveWorktree_NilResolver(t *testing.T) {
	result := &Result{ProjectRoot: "/some/path"}

	resolved, err := ResolveWorktree(result, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.ProjectRoot != "/some/path" {
		t.Errorf("expected original result, got %q", resolved.ProjectRoot)
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
