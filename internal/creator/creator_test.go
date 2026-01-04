package creator

import (
	"os"
	"testing"

	"github.com/amterp/kan/internal/git"
)

func TestGetCreator_KanUserEnvVar(t *testing.T) {
	// KAN_USER should take highest priority
	originalKanUser := os.Getenv("KAN_USER")
	originalUser := os.Getenv("USER")
	defer func() {
		os.Setenv("KAN_USER", originalKanUser)
		os.Setenv("USER", originalUser)
	}()

	os.Setenv("KAN_USER", "kan-test-user")
	os.Setenv("USER", "os-user")

	result, err := GetCreator(git.NewClient())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "kan-test-user" {
		t.Errorf("expected 'kan-test-user', got %q", result)
	}
}

func TestGetCreator_FallsBackToUser(t *testing.T) {
	// When KAN_USER is not set and git isn't available/configured,
	// should fall back to $USER
	originalKanUser := os.Getenv("KAN_USER")
	originalUser := os.Getenv("USER")
	defer func() {
		os.Setenv("KAN_USER", originalKanUser)
		os.Setenv("USER", originalUser)
	}()

	os.Unsetenv("KAN_USER")
	os.Setenv("USER", "fallback-user")

	// Pass nil git client to simulate git not being available
	result, err := GetCreator(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fallback-user" {
		t.Errorf("expected 'fallback-user', got %q", result)
	}
}

func TestGetCreator_ErrorWhenNothingAvailable(t *testing.T) {
	originalKanUser := os.Getenv("KAN_USER")
	originalUser := os.Getenv("USER")
	defer func() {
		os.Setenv("KAN_USER", originalKanUser)
		os.Setenv("USER", originalUser)
	}()

	os.Unsetenv("KAN_USER")
	os.Unsetenv("USER")

	// Pass nil git client
	_, err := GetCreator(nil)
	if err == nil {
		t.Fatal("expected error when no creator source available")
	}

	// Error message should be helpful
	expectedSubstring := "cannot determine creator"
	if !contains(err.Error(), expectedSubstring) {
		t.Errorf("expected error to contain %q, got %q", expectedSubstring, err.Error())
	}
}

func TestGetCreator_KanUserTakesPrecedenceOverGit(t *testing.T) {
	// Even when git is available, KAN_USER should win
	originalKanUser := os.Getenv("KAN_USER")
	defer func() {
		os.Setenv("KAN_USER", originalKanUser)
	}()

	os.Setenv("KAN_USER", "explicit-user")

	// Real git client - but KAN_USER should still take precedence
	result, err := GetCreator(git.NewClient())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "explicit-user" {
		t.Errorf("expected 'explicit-user', got %q", result)
	}
}

func TestGetCreator_EmptyKanUserIsIgnored(t *testing.T) {
	originalKanUser := os.Getenv("KAN_USER")
	originalUser := os.Getenv("USER")
	defer func() {
		os.Setenv("KAN_USER", originalKanUser)
		os.Setenv("USER", originalUser)
	}()

	os.Setenv("KAN_USER", "") // Empty string
	os.Setenv("USER", "os-user")

	result, err := GetCreator(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty KAN_USER should be treated as unset, fall back to USER
	if result != "os-user" {
		t.Errorf("expected 'os-user', got %q", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
