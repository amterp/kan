package version

import (
	"fmt"
	"testing"
)

func TestFormatBoardSchema(t *testing.T) {
	tests := []struct {
		version  int
		expected string
	}{
		{1, "board/1"},
		{2, "board/2"},
		{10, "board/10"},
	}
	for _, tt := range tests {
		got := FormatBoardSchema(tt.version)
		if got != tt.expected {
			t.Errorf("FormatBoardSchema(%d) = %q, want %q", tt.version, got, tt.expected)
		}
	}
}

func TestFormatGlobalSchema(t *testing.T) {
	tests := []struct {
		version  int
		expected string
	}{
		{1, "global/1"},
		{2, "global/2"},
		{10, "global/10"},
	}
	for _, tt := range tests {
		got := FormatGlobalSchema(tt.version)
		if got != tt.expected {
			t.Errorf("FormatGlobalSchema(%d) = %q, want %q", tt.version, got, tt.expected)
		}
	}
}

func TestParseBoardVersion(t *testing.T) {
	tests := []struct {
		schema    string
		expected  int
		expectErr bool
	}{
		{"board/1", 1, false},
		{"board/2", 2, false},
		{"board/10", 10, false},
		{"global/1", 0, true},  // Wrong prefix
		{"board/", 0, true},    // Missing version
		{"board/abc", 0, true}, // Invalid version
		{"board/0", 0, true},   // Version must be >= 1
		{"board/-1", 0, true},  // Negative version
		{"", 0, true},          // Empty
		{"1", 0, true},         // No prefix
	}
	for _, tt := range tests {
		got, err := ParseBoardVersion(tt.schema)
		if tt.expectErr {
			if err == nil {
				t.Errorf("ParseBoardVersion(%q) expected error, got %d", tt.schema, got)
			}
		} else {
			if err != nil {
				t.Errorf("ParseBoardVersion(%q) unexpected error: %v", tt.schema, err)
			} else if got != tt.expected {
				t.Errorf("ParseBoardVersion(%q) = %d, want %d", tt.schema, got, tt.expected)
			}
		}
	}
}

func TestParseGlobalVersion(t *testing.T) {
	tests := []struct {
		schema    string
		expected  int
		expectErr bool
	}{
		{"global/1", 1, false},
		{"global/2", 2, false},
		{"board/1", 0, true},  // Wrong prefix
		{"global/", 0, true},  // Missing version
		{"global/0", 0, true}, // Version must be >= 1
	}
	for _, tt := range tests {
		got, err := ParseGlobalVersion(tt.schema)
		if tt.expectErr {
			if err == nil {
				t.Errorf("ParseGlobalVersion(%q) expected error, got %d", tt.schema, got)
			}
		} else {
			if err != nil {
				t.Errorf("ParseGlobalVersion(%q) unexpected error: %v", tt.schema, err)
			} else if got != tt.expected {
				t.Errorf("ParseGlobalVersion(%q) = %d, want %d", tt.schema, got, tt.expected)
			}
		}
	}
}

func TestCurrentSchemas(t *testing.T) {
	// Verify current schema functions return expected format
	boardSchema := CurrentBoardSchema()
	if boardSchema != "board/4" {
		t.Errorf("CurrentBoardSchema() = %q, want %q", boardSchema, "board/4")
	}

	globalSchema := CurrentGlobalSchema()
	if globalSchema != "global/1" {
		t.Errorf("CurrentGlobalSchema() = %q, want %q", globalSchema, "global/1")
	}
}

func TestSchemaVersionError(t *testing.T) {
	// Test missing version error message
	err := MissingCardVersion("/path/to/card.json")
	if err == nil {
		t.Fatal("MissingCardVersion should return error")
	}
	msg := err.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}

	// Test invalid version with upgrade message
	err = InvalidCardVersion("/path/to/card.json", 2, 1)
	msg = err.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}
}

// TestMinKanVersionCompleteness ensures all current schema versions have
// corresponding entries in MinKanVersion. This catches the case where someone
// bumps a version constant but forgets to update MinKanVersion.
func TestMinKanVersionCompleteness(t *testing.T) {
	requiredKeys := []string{
		fmt.Sprintf("card/%d", CurrentCardVersion),
		fmt.Sprintf("board/%d", CurrentBoardVersion),
		fmt.Sprintf("global/%d", CurrentGlobalVersion),
	}

	for _, key := range requiredKeys {
		if _, ok := MinKanVersion[key]; !ok {
			t.Errorf("MinKanVersion missing entry for %q - update MinKanVersion when bumping schema versions", key)
		}
	}
}
