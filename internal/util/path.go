package util

import (
	"os"
	"strings"
)

// PrettyPath abbreviates the user's home directory to ~ for friendlier output.
func PrettyPath(p string) string {
	home, err := os.UserHomeDir()
	// Require a path-segment boundary so /home/alice doesn't match
	// /home/alicewonderland.
	if err == nil && home != "" && (p == home || strings.HasPrefix(p, home+string(os.PathSeparator))) {
		return "~" + p[len(home):]
	}
	return p
}
