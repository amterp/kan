package git

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client provides git operations.
type Client struct{}

// NewClient creates a new git client.
func NewClient() *Client {
	return &Client{}
}

// GetUserName returns the configured git user.name.
// Returns empty string if git is unavailable or user.name is not configured.
func (c *Client) GetUserName() string {
	cmd := exec.Command("git", "config", "user.name")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetRepoRoot returns the repository root directory.
// Returns an error if not in a git repository.
func (c *Client) GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("not in a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// IsRepo returns true if the current directory is inside a git repository.
func (c *Client) IsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// IsRepoAt returns true if the given directory is inside a git repository.
func (c *Client) IsRepoAt(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// IsWorktree returns true if the current directory is inside a git worktree
// (not the main worktree). Returns false if not in a git repo.
func (c *Client) IsWorktree() bool {
	commonDir, err := c.gitRevParse("--git-common-dir")
	if err != nil {
		return false
	}
	gitDir, err := c.gitRevParse("--git-dir")
	if err != nil {
		return false
	}

	// Resolve to absolute paths for reliable comparison
	absCommon, err := filepath.Abs(commonDir)
	if err != nil {
		return false
	}
	absGit, err := filepath.Abs(gitDir)
	if err != nil {
		return false
	}

	// In a worktree, --git-dir is something like <main>/.git/worktrees/<name>
	// while --git-common-dir is <main>/.git
	return absCommon != absGit
}

// GetMainWorktreeRoot returns the root directory of the main worktree.
// Returns an error if not in a git repo or not in a worktree.
func (c *Client) GetMainWorktreeRoot() (string, error) {
	if !c.IsWorktree() {
		return "", errors.New("not in a git worktree")
	}

	commonDir, err := c.gitRevParse("--git-common-dir")
	if err != nil {
		return "", errors.New("failed to determine git common dir")
	}

	absCommon, err := filepath.Abs(commonDir)
	if err != nil {
		return "", errors.New("failed to resolve git common dir")
	}

	// --git-common-dir returns the main repo's .git directory.
	// The main worktree root is its parent.
	return filepath.Dir(absCommon), nil
}

// StatusPorcelain returns `git status --porcelain` output for the given paths.
// Empty output means no changes. dir sets the working directory for the command.
func (c *Client) StatusPorcelain(dir string, paths ...string) (string, error) {
	args := append([]string{"status", "--porcelain", "--"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status failed: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Add stages the given paths. dir sets the working directory for the command.
func (c *Client) Add(dir string, paths ...string) error {
	args := append([]string{"add", "--"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Commit creates a commit with the given message. When paths are provided,
// only those paths are included in the commit - other staged changes are left
// untouched. This is equivalent to running `git commit -m <msg> -- <paths>`,
// which scopes the commit to the specified paths without the --only flag.
// dir sets the working directory for the command.
func (c *Client) Commit(dir string, message string, paths ...string) error {
	args := []string{"commit", "-m", message}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) gitRevParse(arg string) (string, error) {
	cmd := exec.Command("git", "rev-parse", arg)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
