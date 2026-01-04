package git

import (
	"errors"
	"os/exec"
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
