package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/amterp/kan/internal/model"
)

func TestFindMatchingHooks(t *testing.T) {
	service := NewHookService("/tmp")

	hooks := []model.PatternHook{
		{Name: "jira", PatternTitle: "^[A-Z]+-\\d+$", Command: "echo"},
		{Name: "github", PatternTitle: "#\\d+", Command: "echo"},
		{Name: "all", PatternTitle: ".*", Command: "echo"},
		{Name: "invalid", PatternTitle: "[invalid", Command: "echo"}, // Invalid regex
	}

	tests := []struct {
		name     string
		title    string
		expected []string // Expected hook names
	}{
		{"JIRA ticket", "PROJ-123", []string{"jira", "all"}},
		{"GitHub issue", "Fix #456", []string{"github", "all"}},
		{"Plain text", "Fix the bug", []string{"all"}},
		{"Empty title", "", []string{"all"}},
		{"Multiple JIRA", "PROJ-123-456", []string{"all"}}, // Doesn't match anchored JIRA pattern
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := service.FindMatchingHooks(hooks, tt.title)

			if len(matches) != len(tt.expected) {
				t.Errorf("FindMatchingHooks(%q) returned %d hooks, want %d",
					tt.title, len(matches), len(tt.expected))
				return
			}

			for i, match := range matches {
				if match.Name != tt.expected[i] {
					t.Errorf("FindMatchingHooks(%q)[%d].Name = %q, want %q",
						tt.title, i, match.Name, tt.expected[i])
				}
			}
		})
	}
}

func TestFindMatchingHooks_NoHooks(t *testing.T) {
	service := NewHookService("/tmp")

	matches := service.FindMatchingHooks(nil, "any title")
	if len(matches) != 0 {
		t.Errorf("FindMatchingHooks with nil hooks should return empty, got %d", len(matches))
	}

	matches = service.FindMatchingHooks([]model.PatternHook{}, "any title")
	if len(matches) != 0 {
		t.Errorf("FindMatchingHooks with empty hooks should return empty, got %d", len(matches))
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tilde prefix", "~/bin/script.sh", filepath.Join(home, "bin/script.sh")},
		{"no tilde", "/usr/bin/script.sh", "/usr/bin/script.sh"},
		{"tilde only", "~", "~"}, // Only ~/... is expanded
		{"tilde in middle", "/home/~user", "/home/~user"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTilde(tt.input)
			if result != tt.expected {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExecuteHook_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()
	service := NewHookService(tmpDir)

	hook := model.PatternHook{
		Name:         "test-echo",
		PatternTitle: ".*",
		Command:      "echo",
		Timeout:      5,
	}

	result := service.ExecuteHook(hook, "card-123", "main")

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	// echo prints the arguments
	if result.Stdout != "card-123 main" {
		t.Errorf("Expected stdout 'card-123 main', got %q", result.Stdout)
	}
	if result.HookName != "test-echo" {
		t.Errorf("Expected hook name 'test-echo', got %q", result.HookName)
	}
}

func TestExecuteHook_Failure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()
	service := NewHookService(tmpDir)

	hook := model.PatternHook{
		Name:         "test-fail",
		PatternTitle: ".*",
		Command:      "false", // Unix command that always exits with 1
		Timeout:      5,
	}

	result := service.ExecuteHook(hook, "card-123", "main")

	if result.Success {
		t.Error("Expected failure, got success")
	}
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
}

func TestExecuteHook_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewHookService(tmpDir)

	hook := model.PatternHook{
		Name:         "test-notfound",
		PatternTitle: ".*",
		Command:      "/nonexistent/command/path",
		Timeout:      5,
	}

	result := service.ExecuteHook(hook, "card-123", "main")

	if result.Success {
		t.Error("Expected failure for nonexistent command")
	}
	if result.Error == nil {
		t.Error("Expected error for nonexistent command")
	}
	// The command is what an operator needs to act on an exec failure, so it must
	// survive on the result rather than only living in the config.
	if result.Command != "/nonexistent/command/path" {
		t.Errorf("Expected command to be recorded, got %q", result.Command)
	}
}

// A timed-out hook is SIGKILLed, and a signalled process still yields an *exec.ExitError.
// Reporting must therefore name the timeout rather than surfacing a bare "signal: killed".
//
// The hook here spawns a child that outlives it. The child inherits the output pipes, so
// without a WaitDelay the read would block for the child's full lifetime and the timeout
// would bound nothing - hence the wall-clock assertion.
func TestExecuteHook_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()
	service := NewHookService(tmpDir)

	script := filepath.Join(tmpDir, "hang.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	hook := model.PatternHook{
		Name:         "test-timeout",
		PatternTitle: ".*",
		Command:      script,
		Timeout:      1,
	}

	start := time.Now()
	result := service.ExecuteHook(hook, "card-123", "main")
	elapsed := time.Since(start)

	if result.Success {
		t.Error("Expected failure for timed-out hook")
	}
	if result.Error == nil {
		t.Fatal("Expected error for timed-out hook")
	}
	if !strings.Contains(result.Error.Error(), "timed out") {
		t.Errorf("Expected a timeout error, got %q", result.Error.Error())
	}
	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1 for timeout, got %d", result.ExitCode)
	}
	// Must return shortly after the deadline, not after the orphaned child's 30s sleep.
	if maxWait := 10 * time.Second; elapsed > maxWait {
		t.Errorf("1s timeout took %v to return, want under %v - the timeout is not bounding execution", elapsed, maxWait)
	}
}

// A hook that backgrounds work and exits 0 is a legitimate fire-and-forget pattern. The
// lingering child keeps the output pipes open, which trips WaitDelay - but the hook
// itself succeeded, so it must not be reported as a failure.
func TestExecuteHook_BackgroundChildStillSucceeds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()
	service := NewHookService(tmpDir)

	script := filepath.Join(tmpDir, "fire-and-forget.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n(sleep 5) &\necho done\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	result := service.ExecuteHook(model.PatternHook{
		Name:         "fire-and-forget",
		PatternTitle: ".*",
		Command:      script,
		Timeout:      30,
	}, "card-123", "main")

	if !result.Success {
		t.Errorf("Expected success for a hook that exits 0 while a child lives on, got error: %v", result.Error)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if result.Error != nil {
		t.Errorf("Expected no error, got %v", result.Error)
	}
	if result.Stdout != "done" {
		t.Errorf("Expected stdout %q, got %q", "done", result.Stdout)
	}
}

func TestExecuteHook_DefaultTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewHookService(tmpDir)

	hook := model.PatternHook{
		Name:         "test-timeout",
		PatternTitle: ".*",
		Command:      "echo",
		Timeout:      0, // Should use default
	}

	// Just verify it runs (default timeout is 30s, echo is instant)
	result := service.ExecuteHook(hook, "card-123", "main")
	if result.Error != nil && runtime.GOOS != "windows" {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}

func TestExecuteHooks_Multiple(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	tmpDir := t.TempDir()
	service := NewHookService(tmpDir)

	hooks := []model.PatternHook{
		{Name: "hook1", PatternTitle: ".*", Command: "echo", Timeout: 5},
		{Name: "hook2", PatternTitle: ".*", Command: "echo", Timeout: 5},
	}

	results := service.ExecuteHooks(hooks, "card-123", "main")

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Success {
			t.Errorf("Hook %d failed: %v", i, result.Error)
		}
	}
}

func TestExecuteHooks_Empty(t *testing.T) {
	service := NewHookService("/tmp")

	results := service.ExecuteHooks(nil, "card-123", "main")
	if len(results) != 0 {
		t.Errorf("Expected empty results for nil hooks, got %d", len(results))
	}

	results = service.ExecuteHooks([]model.PatternHook{}, "card-123", "main")
	if len(results) != 0 {
		t.Errorf("Expected empty results for empty hooks, got %d", len(results))
	}
}
