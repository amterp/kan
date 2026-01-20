package model

import (
	"strings"
)

// IconType constants for favicon configuration.
const (
	IconTypeLetter = "letter"
	IconTypeEmoji  = "emoji"
)

// ProjectConfig represents the project-level configuration.
// Stored at .kan/config.toml
// Schema changes require a version bump—see internal/version/version.go.
type ProjectConfig struct {
	KanSchema string        `toml:"kan_schema" json:"kan_schema"`
	ID        string        `toml:"id" json:"id"`
	Name      string        `toml:"name" json:"name"`
	Favicon   FaviconConfig `toml:"favicon" json:"favicon"`
}

// FaviconConfig holds the favicon appearance settings.
type FaviconConfig struct {
	Background string `toml:"background" json:"background"` // Hex color
	IconType   string `toml:"icon_type" json:"icon_type"`   // "letter" or "emoji"
	Letter     string `toml:"letter" json:"letter"`         // Single letter (if icon_type="letter")
	Emoji      string `toml:"emoji" json:"emoji"`           // Unicode emoji (if icon_type="emoji")
}

// FaviconColors is a palette of vibrant, distinct colors for favicon backgrounds.
// Designed to be easily distinguishable in browser tabs.
var FaviconColors = []string{
	"#3b82f6", // blue
	"#ef4444", // red
	"#10b981", // emerald
	"#f59e0b", // amber
	"#8b5cf6", // violet
	"#ec4899", // pink
	"#06b6d4", // cyan
	"#f97316", // orange
	"#84cc16", // lime
	"#6366f1", // indigo
}

// ColorFromID returns a deterministic color from the favicon palette based on the project ID.
// This ensures the color stays consistent even if the project name changes.
func ColorFromID(id string) string {
	if id == "" {
		return FaviconColors[0] // Fixed fallback for empty ID
	}
	hash := 0
	for _, r := range id {
		hash = hash*31 + int(r)
	}
	if hash < 0 {
		hash = -hash
	}
	return FaviconColors[hash%len(FaviconColors)]
}

// DefaultFaviconConfig creates a default favicon config for a project.
// Uses the first letter of the project name (uppercased) and derives color from ID.
func DefaultFaviconConfig(projectID, projectName string) FaviconConfig {
	letter := "K" // Fallback
	if len(projectName) > 0 {
		letter = strings.ToUpper(string([]rune(projectName)[0]))
	}

	return FaviconConfig{
		Background: ColorFromID(projectID),
		IconType:   IconTypeLetter,
		Letter:     letter,
		Emoji:      "",
	}
}
