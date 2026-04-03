package cli

import (
	"testing"
)

func TestStringToColor_CrossImplementation(t *testing.T) {
	// Expected values computed from web/src/utils/badgeColors.ts
	tests := []struct {
		input    string
		expected string
	}{
		{"bug", "#c2410c"},
		{"feature", "#9f1239"},
		{"enhancement", "#0f766e"},
		{"urgent", "#0e7490"},
		{"backend", "#6b21a8"},
		{"frontend", "#c2410c"},
		{"api", "#0f766e"},
		{"blocked", "#991b1b"},
		{"in-progress", "#92400e"},
		{"wontfix", "#1e40af"},
	}
	for _, tt := range tests {
		got := stringToColor(tt.input)
		if got != tt.expected {
			t.Errorf("stringToColor(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestStringToColor_CaseInsensitive(t *testing.T) {
	cases := [][]string{
		{"Bug", "bug", "BUG"},
		{"Feature", "feature", "FEATURE"},
	}
	for _, group := range cases {
		first := stringToColor(group[0])
		for _, variant := range group[1:] {
			if got := stringToColor(variant); got != first {
				t.Errorf("stringToColor(%q) = %q, want %q (same as %q)", variant, got, first, group[0])
			}
		}
	}
}

func TestStringToColor_EmptyString(t *testing.T) {
	got := stringToColor("")
	if got != badgeColors[0] {
		t.Errorf("stringToColor(\"\") = %q, want %q", got, badgeColors[0])
	}
}

func TestStringToColor_Distribution(t *testing.T) {
	words := []string{"Bug", "Feature", "Enhancement", "Critical", "Low", "Medium", "High", "UI", "Backend", "API"}
	seen := make(map[string]bool)
	for _, w := range words {
		seen[stringToColor(w)] = true
	}
	if len(seen) < 5 {
		t.Errorf("expected at least 5 distinct colors from 10 words, got %d", len(seen))
	}
}
