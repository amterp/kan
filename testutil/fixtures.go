package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
)

// TestCard returns a card with sensible test defaults.
func TestCard(id, title string) *model.Card {
	now := time.Now().UnixMilli()
	return &model.Card{
		ID:              id,
		Alias:           "test-card",
		AliasExplicit:   false,
		Title:           title,
		Column:          "backlog",
		Creator:         "tester",
		CreatedAtMillis: now,
		UpdatedAtMillis: now,
	}
}

// TestBoardConfig returns a board config with sensible test defaults.
func TestBoardConfig(name string) *model.BoardConfig {
	return &model.BoardConfig{
		ID:            "test-board-id",
		Name:          name,
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
	}
}

// TempKanDir creates a temporary directory with a .kan structure for testing.
// Returns the temp dir path and a cleanup function.
func TempKanDir(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "kan-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create .kan/boards structure
	boardsDir := filepath.Join(dir, ".kan", "boards")
	if err := os.MkdirAll(boardsDir, 0755); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create boards dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// TempGitRepo creates a temporary git repository for testing.
// Returns the repo path and a cleanup function.
func TempGitRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "kan-git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	cmd.Run()

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// NewTestPaths creates a Paths for testing with the given temp directory.
func NewTestPaths(baseDir string) *config.Paths {
	return config.NewPaths(baseDir, "")
}
