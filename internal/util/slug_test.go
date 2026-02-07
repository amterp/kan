package util

import (
	"slices"
	"strings"
	"testing"
)

func TestSlugWords(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		// Basic cases
		{"Hello World", []string{"hello", "world"}},
		{"Fix login bug", []string{"fix", "login", "bug"}},
		{"Add dark mode", []string{"add", "dark", "mode"}},

		// Special characters
		{"Fix: login issue", []string{"fix", "login", "issue"}},
		{"Add feature (v2)", []string{"add", "feature", "v2"}},
		{"Update README.md", []string{"update", "readme", "md"}},
		{"API v2.0 release", []string{"api", "v2", "0", "release"}},

		// Multiple spaces/hyphens
		{"Multiple   spaces", []string{"multiple", "spaces"}},
		{"Already--hyphenated", []string{"already", "hyphenated"}},
		{"  Leading spaces", []string{"leading", "spaces"}},
		{"Trailing spaces  ", []string{"trailing", "spaces"}},

		// Unicode and accents
		{"Café au lait", []string{"cafe", "au", "lait"}},
		{"Résumé updates", []string{"resume", "updates"}},
		{"naïve implementation", []string{"naive", "implementation"}},

		// Numbers
		{"Issue #123", []string{"issue", "123"}},
		{"Version 2.0.1", []string{"version", "2", "0", "1"}},

		// Edge cases
		{"", nil},
		{"   ", nil},
		{"---", nil},
		{"a", []string{"a"}},
		{"A", []string{"a"}},

		// Long titles produce all words (no truncation here)
		{"This is a very long title that exceeds the limit", []string{
			"this", "is", "a", "very", "long", "title", "that", "exceeds", "the", "limit",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SlugWords(tt.input)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("SlugWords(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugWords_JoinedOutput(t *testing.T) {
	// Verify that joining words produces the expected slug string
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Fix: login issue", "fix-login-issue"},
		{"Café au lait", "cafe-au-lait"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			words := SlugWords(tt.input)
			result := strings.Join(words, "-")
			if result != tt.expected {
				t.Errorf("Join(SlugWords(%q)) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
