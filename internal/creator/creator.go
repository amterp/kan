package creator

import (
	"fmt"
	"os"

	"github.com/amterp/kan/internal/git"
)

// GetCreator returns the username for card creator field using fallback chain:
// 1. $KAN_USER environment variable
// 2. git config user.name (graceful - empty if git unavailable)
// 3. $USER environment variable
// 4. Explicit helpful error
func GetCreator(gitClient *git.Client) (string, error) {
	// 1. KAN_USER env var (highest priority)
	if user := os.Getenv("KAN_USER"); user != "" {
		return user, nil
	}

	// 2. git config user.name (graceful fallback)
	if gitClient != nil {
		if name := gitClient.GetUserName(); name != "" {
			return name, nil
		}
	}

	// 3. $USER env var
	if user := os.Getenv("USER"); user != "" {
		return user, nil
	}

	// 4. Explicit error with helpful message
	return "", fmt.Errorf("cannot determine creator name: set $KAN_USER, configure 'git config user.name', or set $USER")
}
