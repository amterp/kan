package util

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic cases
		{"Hello World", "hello-world"},
		{"Fix login bug", "fix-login-bug"},
		{"Add dark mode", "add-dark-mode"},

		// Special characters
		{"Fix: login issue", "fix-login-issue"},
		{"Add feature (v2)", "add-feature-v2"},
		{"Update README.md", "update-readme-md"},
		{"API v2.0 release", "api-v2-0-release"},

		// Multiple spaces/hyphens
		{"Multiple   spaces", "multiple-spaces"},
		{"Already--hyphenated", "already-hyphenated"},
		{"  Leading spaces", "leading-spaces"},
		{"Trailing spaces  ", "trailing-spaces"},

		// Unicode and accents
		{"Café au lait", "cafe-au-lait"},
		{"Résumé updates", "resume-updates"},
		{"naïve implementation", "naive-implementation"},

		// Numbers
		{"Issue #123", "issue-123"},
		{"Version 2.0.1", "version-2-0-1"},

		// Edge cases
		{"", ""},
		{"   ", ""},
		{"---", ""},
		{"a", "a"},
		{"A", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
