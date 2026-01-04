package resolver

import (
	"testing"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/prompt"
	"github.com/amterp/kan/internal/store"
)

// mockBoardStore implements store.BoardStore for testing.
type mockBoardStore struct {
	boards map[string]*model.BoardConfig
}

func newMockBoardStore() *mockBoardStore {
	return &mockBoardStore{
		boards: make(map[string]*model.BoardConfig),
	}
}

func (m *mockBoardStore) addBoard(name string) {
	m.boards[name] = &model.BoardConfig{
		ID:   name + "-id",
		Name: name,
	}
}

func (m *mockBoardStore) Create(config *model.BoardConfig) error {
	return nil
}

func (m *mockBoardStore) Get(boardName string) (*model.BoardConfig, error) {
	if cfg, ok := m.boards[boardName]; ok {
		return cfg, nil
	}
	return nil, kanerr.BoardNotFound(boardName)
}

func (m *mockBoardStore) Update(config *model.BoardConfig) error {
	return nil
}

func (m *mockBoardStore) List() ([]string, error) {
	var names []string
	for name := range m.boards {
		names = append(names, name)
	}
	return names, nil
}

func (m *mockBoardStore) Exists(boardName string) bool {
	_, ok := m.boards[boardName]
	return ok
}

var _ store.BoardStore = (*mockBoardStore)(nil)

// mockGlobalStore implements store.GlobalStore for testing.
type mockGlobalStore struct {
	config *model.GlobalConfig
}

func newMockGlobalStore() *mockGlobalStore {
	return &mockGlobalStore{
		config: &model.GlobalConfig{
			Repos: make(map[string]model.RepoConfig),
		},
	}
}

func (m *mockGlobalStore) setDefaultBoard(repoPath, board string) {
	m.config.Repos[repoPath] = model.RepoConfig{DefaultBoard: board}
}

func (m *mockGlobalStore) Load() (*model.GlobalConfig, error) {
	return m.config, nil
}

func (m *mockGlobalStore) Save(config *model.GlobalConfig) error {
	m.config = config
	return nil
}

func (m *mockGlobalStore) EnsureExists() error {
	return nil
}

var _ store.GlobalStore = (*mockGlobalStore)(nil)

// mockPrompter implements prompt.Prompter for testing.
type mockPrompter struct {
	selectResult string
	selectError  error
}

func (m *mockPrompter) Select(title string, options []string) (string, error) {
	if m.selectError != nil {
		return "", m.selectError
	}
	return m.selectResult, nil
}

func (m *mockPrompter) Input(title string, defaultValue string) (string, error) {
	return "", nil
}

func (m *mockPrompter) Confirm(title string, defaultValue bool) (bool, error) {
	return false, nil
}

func (m *mockPrompter) MultiSelect(title string, options []string) ([]string, error) {
	return nil, nil
}

var _ prompt.Prompter = (*mockPrompter)(nil)

// ============================================================================
// BoardResolver Tests
// ============================================================================

func TestBoardResolver_Resolve_ExplicitBoard(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")
	boardStore.addBoard("feature")

	resolver := NewBoardResolver(boardStore, newMockGlobalStore(), &prompt.NoopPrompter{}, "/repo")

	// Explicit board should be used directly
	board, err := resolver.Resolve("main", false)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if board != "main" {
		t.Errorf("Expected 'main', got %q", board)
	}
}

func TestBoardResolver_Resolve_ExplicitBoard_NotFound(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")

	resolver := NewBoardResolver(boardStore, newMockGlobalStore(), &prompt.NoopPrompter{}, "/repo")

	_, err := resolver.Resolve("nonexistent", false)
	if err == nil {
		t.Fatal("Expected error for nonexistent board")
	}
	expected := `board "nonexistent" not found`
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}

func TestBoardResolver_Resolve_SingleBoard(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("only-board")

	resolver := NewBoardResolver(boardStore, newMockGlobalStore(), &prompt.NoopPrompter{}, "/repo")

	// With only one board, it should be auto-selected
	board, err := resolver.Resolve("", false)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if board != "only-board" {
		t.Errorf("Expected 'only-board', got %q", board)
	}
}

func TestBoardResolver_Resolve_NoBoards(t *testing.T) {
	boardStore := newMockBoardStore()
	// No boards added

	resolver := NewBoardResolver(boardStore, newMockGlobalStore(), &prompt.NoopPrompter{}, "/repo")

	_, err := resolver.Resolve("", false)
	if err == nil {
		t.Fatal("Expected error when no boards exist")
	}
	expected := "no boards found; run 'kan init' first"
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}

func TestBoardResolver_Resolve_DefaultFromConfig(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")
	boardStore.addBoard("feature")
	boardStore.addBoard("bugfix")

	globalStore := newMockGlobalStore()
	globalStore.setDefaultBoard("/repo", "feature")

	resolver := NewBoardResolver(boardStore, globalStore, &prompt.NoopPrompter{}, "/repo")

	// With multiple boards and a default configured, should use default
	board, err := resolver.Resolve("", false)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if board != "feature" {
		t.Errorf("Expected 'feature' (default), got %q", board)
	}
}

func TestBoardResolver_Resolve_DefaultBoard_DoesNotExist(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")
	boardStore.addBoard("feature")

	globalStore := newMockGlobalStore()
	globalStore.setDefaultBoard("/repo", "deleted-board") // Board no longer exists

	resolver := NewBoardResolver(boardStore, globalStore, &prompt.NoopPrompter{}, "/repo")

	// Default board doesn't exist, should fall back to prompting (which fails non-interactive)
	_, err := resolver.Resolve("", false)
	if err == nil {
		t.Fatal("Expected error when default board doesn't exist and not interactive")
	}
}

func TestBoardResolver_Resolve_MultipleBoards_NonInteractive(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")
	boardStore.addBoard("feature")

	resolver := NewBoardResolver(boardStore, newMockGlobalStore(), &prompt.NoopPrompter{}, "/repo")

	// Multiple boards, no default, non-interactive
	_, err := resolver.Resolve("", false)
	if err == nil {
		t.Fatal("Expected error for multiple boards in non-interactive mode")
	}
	expected := "multiple boards exist; specify with -b or set default_board in config"
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}

func TestBoardResolver_Resolve_MultipleBoards_Interactive(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")
	boardStore.addBoard("feature")

	prompter := &mockPrompter{selectResult: "feature"}

	resolver := NewBoardResolver(boardStore, newMockGlobalStore(), prompter, "/repo")

	// Multiple boards, interactive, should prompt and return selected
	board, err := resolver.Resolve("", true)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if board != "feature" {
		t.Errorf("Expected 'feature' (selected), got %q", board)
	}
}

func TestBoardResolver_Resolve_Interactive_PromptError(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")
	boardStore.addBoard("feature")

	prompter := &mockPrompter{selectError: prompt.ErrNonInteractive}

	resolver := NewBoardResolver(boardStore, newMockGlobalStore(), prompter, "/repo")

	_, err := resolver.Resolve("", true)
	if err == nil {
		t.Fatal("Expected error when prompter fails")
	}
	if err != prompt.ErrNonInteractive {
		t.Errorf("Expected ErrNonInteractive, got %v", err)
	}
}

func TestBoardResolver_Resolve_DifferentRepoPaths(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")
	boardStore.addBoard("feature")

	globalStore := newMockGlobalStore()
	globalStore.setDefaultBoard("/other-repo", "feature") // Different repo

	resolver := NewBoardResolver(boardStore, globalStore, &prompt.NoopPrompter{}, "/repo")

	// Default is for different repo, should not apply
	_, err := resolver.Resolve("", false)
	if err == nil {
		t.Fatal("Expected error when default is for different repo")
	}
}

func TestBoardResolver_GetBoardConfig(t *testing.T) {
	boardStore := newMockBoardStore()
	boardStore.addBoard("main")

	resolver := NewBoardResolver(boardStore, newMockGlobalStore(), &prompt.NoopPrompter{}, "/repo")

	cfg, err := resolver.GetBoardConfig("main")
	if err != nil {
		t.Fatalf("GetBoardConfig failed: %v", err)
	}
	if cfg.Name != "main" {
		t.Errorf("Expected board name 'main', got %q", cfg.Name)
	}
}
