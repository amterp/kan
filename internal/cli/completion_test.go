package cli

import "testing"

func TestBoardFromArgs_LongFlagEquals(t *testing.T) {
	args := []string{"kan", "show", "--board=main", "card1"}
	if got := boardFromArgs(args); got != "main" {
		t.Errorf("Expected 'main', got %q", got)
	}
}

func TestBoardFromArgs_ShortFlagEquals(t *testing.T) {
	args := []string{"kan", "show", "-b=feature", "card1"}
	if got := boardFromArgs(args); got != "feature" {
		t.Errorf("Expected 'feature', got %q", got)
	}
}

func TestBoardFromArgs_LongFlagSpace(t *testing.T) {
	args := []string{"kan", "show", "--board", "main", "card1"}
	if got := boardFromArgs(args); got != "main" {
		t.Errorf("Expected 'main', got %q", got)
	}
}

func TestBoardFromArgs_ShortFlagSpace(t *testing.T) {
	args := []string{"kan", "show", "-b", "feature", "card1"}
	if got := boardFromArgs(args); got != "feature" {
		t.Errorf("Expected 'feature', got %q", got)
	}
}

func TestBoardFromArgs_NoFlag(t *testing.T) {
	args := []string{"kan", "show", "card1"}
	if got := boardFromArgs(args); got != "" {
		t.Errorf("Expected empty string, got %q", got)
	}
}

func TestBoardFromArgs_EmptyArgs(t *testing.T) {
	if got := boardFromArgs(nil); got != "" {
		t.Errorf("Expected empty string for nil args, got %q", got)
	}
	if got := boardFromArgs([]string{}); got != "" {
		t.Errorf("Expected empty string for empty args, got %q", got)
	}
}

func TestBoardFromArgs_EmptyEqualsValue(t *testing.T) {
	// --board= with no value should fall through (not return "")
	args := []string{"kan", "show", "--board=", "card1"}
	if got := boardFromArgs(args); got != "" {
		t.Errorf("Expected empty string for --board= (no value), got %q", got)
	}

	args = []string{"kan", "show", "-b=", "card1"}
	if got := boardFromArgs(args); got != "" {
		t.Errorf("Expected empty string for -b= (no value), got %q", got)
	}
}

func TestBoardFromArgs_FlagAtEnd(t *testing.T) {
	// --board at end with no following value
	args := []string{"kan", "show", "--board"}
	if got := boardFromArgs(args); got != "" {
		t.Errorf("Expected empty string for --board at end, got %q", got)
	}

	args = []string{"kan", "show", "-b"}
	if got := boardFromArgs(args); got != "" {
		t.Errorf("Expected empty string for -b at end, got %q", got)
	}
}

func TestBoardFromArgs_FlagAfterPositional(t *testing.T) {
	// Board flag after positional arg (still found)
	args := []string{"kan", "show", "card1", "-b", "main"}
	if got := boardFromArgs(args); got != "main" {
		t.Errorf("Expected 'main', got %q", got)
	}
}

func TestBoardFromArgs_FirstFlagWins(t *testing.T) {
	// Multiple board flags - first one wins
	args := []string{"kan", "show", "-b", "first", "-b", "second"}
	if got := boardFromArgs(args); got != "first" {
		t.Errorf("Expected 'first' (first flag wins), got %q", got)
	}
}
