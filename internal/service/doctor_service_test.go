package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// setupDoctorTest copies test fixtures to a temp directory and returns
// the DoctorService and cleanup function.
func setupDoctorTest(t *testing.T, fixtureName string) (*DoctorService, string, func()) {
	t.Helper()

	fixtureDir := filepath.Join("testdata", "doctor", fixtureName)
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Fatalf("Test fixture not found: %s", fixtureDir)
	}

	tempDir, err := os.MkdirTemp("", "kan-doctor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Isolate HOME so the global-config check reads an (absent) temp config
	// rather than the developer's real ~/.config/kan/config.toml, which would
	// otherwise leak schema-version warnings into these project-scoped tests.
	t.Setenv("HOME", tempDir)

	if err := copyDir(fixtureDir, tempDir); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to copy test fixtures: %v", err)
	}

	paths := config.NewPaths(tempDir, "")
	cardStore := store.NewCardStore(paths)
	service := NewDoctorService(paths, cardStore)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return service, tempDir, cleanup
}

func TestDoctorService_Healthy(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "healthy")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if len(report.Boards) != 1 {
		t.Errorf("Expected 1 board, got %d", len(report.Boards))
	}

	if report.Summary.Errors != 0 {
		t.Errorf("Expected 0 errors, got %d", report.Summary.Errors)
	}

	if report.Summary.Warnings != 0 {
		t.Errorf("Expected 0 warnings, got %d", report.Summary.Warnings)
	}

	// Check board stats
	board := report.Boards[0]
	if board.Name != "main" {
		t.Errorf("Expected board name 'main', got %q", board.Name)
	}
	if board.CardFiles != 2 {
		t.Errorf("Expected 2 card files, got %d", board.CardFiles)
	}
	if board.CardsReferenced != 2 {
		t.Errorf("Expected 2 cards referenced, got %d", board.CardsReferenced)
	}
}

func TestDoctorService_OrphanedCard(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "orphaned-card")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", report.Summary.Errors)
	}

	// Find the orphaned card issue
	found := false
	for _, issue := range report.Issues {
		if issue.Code == CodeOrphanedCard && issue.CardID == "card-orphan" {
			found = true
			if !issue.Fixable {
				t.Error("Orphaned card issue should be fixable")
			}
		}
	}
	if !found {
		t.Error("Expected ORPHANED_CARD issue for card-orphan")
	}
}

func TestDoctorService_OrphanedCard_Fix(t *testing.T) {
	service, tempDir, cleanup := setupDoctorTest(t, "orphaned-card")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Errors != 1 {
		t.Fatalf("Expected 1 error before fix, got %d", report.Summary.Errors)
	}

	// Apply fix
	fixedReport, err := service.Fix(report)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}

	if fixedReport.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fix, got %d", fixedReport.Summary.Fixed)
	}

	// Verify the orphaned card now has a column assigned
	cardPath := filepath.Join(tempDir, ".kan", "boards", "main", "cards", "card-orphan.json")
	data, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("Failed to read card: %v", err)
	}

	cardStr := string(data)
	if !strings.Contains(cardStr, `"column"`) {
		t.Error("Fixed card should have a column field")
	}
}

func TestDoctorService_MissingCard(t *testing.T) {
	// With card-centric storage, "missing card file referenced by board config" can't happen.
	// This fixture now represents a healthy board (all cards have valid columns).
	service, _, cleanup := setupDoctorTest(t, "missing-card")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Errors != 0 {
		t.Errorf("Expected 0 errors, got %d", report.Summary.Errors)
	}
}

func TestDoctorService_DuplicateCard(t *testing.T) {
	// With card-centric storage, duplicate card in multiple columns can't happen.
	// This fixture now represents a healthy board.
	service, _, cleanup := setupDoctorTest(t, "duplicate-card")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Errors != 0 {
		t.Errorf("Expected 0 errors, got %d", report.Summary.Errors)
	}
}

func TestDoctorService_InvalidParent(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "invalid-parent")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Warnings != 1 {
		t.Errorf("Expected 1 warning, got %d", report.Summary.Warnings)
	}

	// Find the invalid parent issue
	found := false
	for _, issue := range report.Issues {
		if issue.Code == CodeInvalidParentRef && issue.CardID == "card-1" {
			found = true
			if !issue.Fixable {
				t.Error("Invalid parent issue should be fixable")
			}
		}
	}
	if !found {
		t.Error("Expected INVALID_PARENT_REF issue for card-1")
	}
}

func TestDoctorService_InvalidParent_Fix(t *testing.T) {
	service, tempDir, cleanup := setupDoctorTest(t, "invalid-parent")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	// Apply fix
	fixedReport, err := service.Fix(report)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}

	if fixedReport.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fix, got %d", fixedReport.Summary.Fixed)
	}

	// Verify the parent field was cleared
	cardPath := filepath.Join(tempDir, ".kan", "boards", "main", "cards", "card-1.json")
	data, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("Failed to read card: %v", err)
	}

	cardStr := string(data)
	// Check for the JSON key pattern, not just "parent" (which appears in the title)
	if strings.Contains(cardStr, `"parent"`) {
		t.Errorf("Fixed card should not contain parent field as JSON key, got: %s", cardStr)
	}
}

func TestDoctorService_SpecificBoard(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "healthy")
	defer cleanup()

	// Test with existing board name
	report, err := service.Diagnose("main")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if len(report.Boards) != 1 {
		t.Errorf("Expected 1 board, got %d", len(report.Boards))
	}

	// Test with non-existing board name
	report, err = service.Diagnose("nonexistent")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if len(report.Boards) != 0 {
		t.Errorf("Expected 0 boards for nonexistent board, got %d", len(report.Boards))
	}
}

func TestDoctorService_HasErrors(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "orphaned-card")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if !report.HasErrors() {
		t.Error("Report should have errors")
	}

	// Healthy board should not have errors
	service2, _, cleanup2 := setupDoctorTest(t, "healthy")
	defer cleanup2()

	report2, err := service2.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report2.HasErrors() {
		t.Error("Healthy report should not have errors")
	}
}

func TestDoctorService_InvalidDefaultColumn(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "invalid-default-column")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Warnings != 1 {
		t.Errorf("Expected 1 warning, got %d", report.Summary.Warnings)
	}

	// Find the invalid default column issue
	found := false
	for _, issue := range report.Issues {
		if issue.Code == CodeInvalidDefaultCol {
			found = true
			if !issue.Fixable {
				t.Error("Invalid default column issue should be fixable")
			}
		}
	}
	if !found {
		t.Error("Expected INVALID_DEFAULT_COLUMN issue")
	}
}

func TestDoctorService_InvalidDefaultColumn_Fix(t *testing.T) {
	service, tempDir, cleanup := setupDoctorTest(t, "invalid-default-column")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	// Apply fix
	fixedReport, err := service.Fix(report)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}

	if fixedReport.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fix, got %d", fixedReport.Summary.Fixed)
	}

	// Verify the default column was reset to first column
	configPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if !strings.Contains(string(data), `default_column = "backlog"`) {
		t.Error("Expected default_column to be reset to 'backlog'")
	}
}

func TestDoctorService_MalformedCard(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "malformed-card")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", report.Summary.Errors)
	}

	// Find the malformed card issue
	found := false
	for _, issue := range report.Issues {
		if issue.Code == CodeMalformedCard && issue.CardID == "card-1" {
			found = true
			if issue.Fixable {
				t.Error("Malformed card issue should NOT be fixable")
			}
		}
	}
	if !found {
		t.Error("Expected MALFORMED_CARD issue for card-1")
	}
}

func TestDoctorService_SchemaOutdated(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "schema-outdated")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Warnings != 1 {
		t.Errorf("Expected 1 warning, got %d", report.Summary.Warnings)
	}

	// Find the schema outdated issue
	found := false
	for _, issue := range report.Issues {
		if issue.Code == CodeSchemaOutdated {
			found = true
			if issue.Fixable {
				t.Error("Schema outdated issue should NOT be fixable by doctor")
			}
			if !strings.Contains(issue.FixAction, "migrate") {
				t.Error("Schema outdated should suggest running migrate")
			}
		}
	}
	if !found {
		t.Error("Expected SCHEMA_OUTDATED issue")
	}
}

// Hooks run with the project root as their working directory, so doctor must resolve
// relative commands against it too. Bare relative paths like ".kan/hooks/x.rad" - the
// form the docs recommend - previously escaped every check.
func TestDoctorService_PatternHooks(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, ".kan", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}

	executable := filepath.Join(hooksDir, "runnable.sh")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("Failed to write hook: %v", err)
	}
	notExecutable := filepath.Join(hooksDir, "plain.sh")
	if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("Failed to write hook: %v", err)
	}
	spaced := filepath.Join(hooksDir, "with space.sh")
	if err := os.WriteFile(spaced, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("Failed to write hook: %v", err)
	}

	service := NewDoctorService(config.NewPaths(tmpDir, ""), nil)

	tests := []struct {
		name     string
		hook     model.PatternHook
		wantCode string // empty means no issue expected
		unixOnly bool   // relies on the executable bit, which Windows doesn't report
	}{
		{
			name: "relative path, executable",
			hook: model.PatternHook{Name: "ok", PatternTitle: ".*", Command: ".kan/hooks/runnable.sh"},
		},
		{
			name:     "relative path, not executable",
			hook:     model.PatternHook{Name: "perm", PatternTitle: ".*", Command: ".kan/hooks/plain.sh"},
			wantCode: CodeHookNotExecutable,
			unixOnly: true,
		},
		{
			name:     "relative path, missing",
			hook:     model.PatternHook{Name: "gone", PatternTitle: ".*", Command: ".kan/hooks/absent.sh"},
			wantCode: CodeMissingHookFile,
		},
		{
			name:     "absolute path, missing",
			hook:     model.PatternHook{Name: "abs", PatternTitle: ".*", Command: "/nonexistent/hook.sh"},
			wantCode: CodeMissingHookFile,
		},
		{
			name: "bare command on PATH",
			hook: model.PatternHook{Name: "bare", PatternTitle: ".*", Command: "sh"},
		},
		{
			name:     "bare command not on PATH",
			hook:     model.PatternHook{Name: "nope", PatternTitle: ".*", Command: "kan-definitely-not-a-real-binary"},
			wantCode: CodeMissingHookFile,
		},
		{
			name:     "command with arguments",
			hook:     model.PatternHook{Name: "args", PatternTitle: ".*", Command: "python script.py"},
			wantCode: CodeHookCommandArgs,
		},
		{
			name:     "path-like command with arguments",
			hook:     model.PatternHook{Name: "relargs", PatternTitle: ".*", Command: ".kan/hooks/runnable.sh --verbose"},
			wantCode: CodeHookCommandArgs,
		},
		{
			// A space in a path is not an argument. exec runs this fine, so doctor must not
			// cry wolf - "Application Support" and "/Users/John Doe" are everyday paths.
			name: "existing path containing a space",
			hook: model.PatternHook{Name: "spacey", PatternTitle: ".*", Command: ".kan/hooks/with space.sh"},
		},
		{
			name:     "command is a directory",
			hook:     model.PatternHook{Name: "dir", PatternTitle: ".*", Command: ".kan/hooks"},
			wantCode: CodeHookNotExecutable,
		},
		{
			name:     "invalid regex",
			hook:     model.PatternHook{Name: "regex", PatternTitle: "[invalid", Command: ".kan/hooks/runnable.sh"},
			wantCode: CodeInvalidPatternHook,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unixOnly && runtime.GOOS == "windows" {
				t.Skip("Executable bit is not meaningful on Windows")
			}

			report := &DiagnosticReport{}
			cfg := &model.BoardConfig{PatternHooks: []model.PatternHook{tt.hook}}
			service.checkPatternHooks(report, "main", cfg)

			if tt.wantCode == "" {
				if len(report.Issues) != 0 {
					t.Errorf("Expected no issues, got %d: %v", len(report.Issues), report.Issues)
				}
				return
			}

			if len(report.Issues) != 1 {
				t.Fatalf("Expected 1 issue with code %s, got %d: %v", tt.wantCode, len(report.Issues), report.Issues)
			}
			if report.Issues[0].Code != tt.wantCode {
				t.Errorf("Expected code %s, got %s (%s)", tt.wantCode, report.Issues[0].Code, report.Issues[0].Message)
			}
		})
	}
}

// Helper functions

func countOccurrences(s, substr string) int {
	return strings.Count(s, substr)
}
