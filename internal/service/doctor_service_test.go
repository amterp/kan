package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
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

	// The orphan's old key duplicated card-1's, so it must get a fresh one
	// at the end of its new column.
	orphan, err := service.cardStore.Get("main", "card-orphan")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if orphan.Position != "a1" {
		t.Errorf("orphan position = %q, want a1 (after card-1's a0)", orphan.Position)
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

// setupPositionTest starts from the healthy fixture and replaces its backlog
// with cards holding the given positions, in the given order of IDs p00, p01, ...
func setupPositionTest(t *testing.T, positions []string) (*DoctorService, func()) {
	t.Helper()
	service, tempDir, cleanup := setupDoctorTest(t, "healthy")
	if err := os.Remove(filepath.Join(tempDir, ".kan", "boards", "main", "cards", "card-1.json")); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	for i, pos := range positions {
		card := &model.Card{
			Version:         version.CurrentCardVersion,
			ID:              fmt.Sprintf("p%02d", i),
			Alias:           fmt.Sprintf("p%02d", i),
			Title:           fmt.Sprintf("Card %d", i),
			Column:          "backlog",
			Position:        pos,
			CreatedAtMillis: 1700000000000,
			UpdatedAtMillis: 1700000000000,
		}
		if err := service.cardStore.Create("main", card); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}
	return service, cleanup
}

func backlogCards(t *testing.T, service *DoctorService) []*model.Card {
	t.Helper()
	cards, err := service.cardStore.List("main")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	return cardsInColumn(cards, "backlog")
}

func positionIssues(report *DiagnosticReport) []Issue {
	var issues []Issue
	for _, issue := range report.Issues {
		switch issue.Code {
		case CodeDuplicatePositionKeys, CodeLegacyPositionKeys, CodeLongPositionKeys:
			issues = append(issues, issue)
		}
	}
	return issues
}

func TestDoctorService_PositionKeys(t *testing.T) {
	long := "a0" + strings.Repeat("V", 12)
	cases := []struct {
		name      string
		positions []string
		wantCode  string // empty means no issue
		wantParts []string
	}{
		{"current keys", []string{"Zz", "a0", "a0V", "a1"}, "", nil},
		{"shared key", []string{"a0", "a1", "a1", "a2"}, CodeDuplicatePositionKeys, []string{"1 shared"}},
		{"older keys", []string{"U", "UU", "a0"}, CodeLegacyPositionKeys, []string{"2 missing or in an older format"}},
		{"missing key", []string{"", "a0"}, CodeLegacyPositionKeys, []string{"1 missing"}},
		{"long key", []string{"a0", long}, CodeLongPositionKeys, []string{"1 longer than 12 characters (longest: 14)"}},
		{"every reason", []string{"U", "U", long}, CodeDuplicatePositionKeys,
			[]string{"1 shared", "2 missing or in an older format", "1 longer than 12"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, cleanup := setupPositionTest(t, tc.positions)
			defer cleanup()

			report, err := service.Diagnose("")
			if err != nil {
				t.Fatalf("Diagnose failed: %v", err)
			}
			issues := positionIssues(report)
			if tc.wantCode == "" {
				if len(issues) != 0 {
					t.Fatalf("expected no position issues, got %+v", issues)
				}
				return
			}
			if len(issues) != 1 {
				t.Fatalf("expected 1 position issue, got %+v", issues)
			}
			issue := issues[0]
			if issue.Code != tc.wantCode {
				t.Errorf("Code = %s, want %s", issue.Code, tc.wantCode)
			}
			if issue.Column != "backlog" || issue.Severity != SeverityWarning || !issue.Fixable {
				t.Errorf("issue = %+v, want a fixable warning on column backlog", issue)
			}
			for _, part := range tc.wantParts {
				if !strings.Contains(issue.Message, part) {
					t.Errorf("Message %q should contain %q", issue.Message, part)
				}
			}
		})
	}
}

func TestDoctorService_PositionKeys_Fix(t *testing.T) {
	bangs := strings.Repeat("!", 37) + "U"
	cases := []struct {
		name         string
		positions    []string
		wantRewrites int
		wantKeys     []string // nil means only check validity
	}{
		// Two branches that each appended one card leave one shared key; only
		// the second card of the pair should change.
		{"one shared key", []string{"a0", "a1", "a1", "a2"}, 1, []string{"a0", "a1", "a1V", "a2"}},
		{"one older key above current ones", []string{"!!U", "a0", "a1", "a2"}, 1, []string{"Zz", "a0", "a1", "a2"}},
		{"mostly older keys", []string{bangs, "0", "E", "U", "UUUU", strings.Repeat("U", 99)}, 6,
			[]string{"a0", "a1", "a2", "a3", "a4", "a5"}},
		{"missing key", []string{"", "a0"}, 1, []string{"Zz", "a0"}},
		{"long key", []string{"a0", "a0" + strings.Repeat("V", 12), "a1"}, 1, []string{"a0", "a0V", "a1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, cleanup := setupPositionTest(t, tc.positions)
			defer cleanup()

			var wantOrder []string
			before := make(map[string]string)
			for _, c := range backlogCards(t, service) {
				wantOrder = append(wantOrder, c.ID)
				before[c.ID] = c.Position
			}

			report, err := service.Diagnose("")
			if err != nil {
				t.Fatalf("Diagnose failed: %v", err)
			}
			issues := positionIssues(report)
			if len(issues) != 1 {
				t.Fatalf("expected 1 position issue, got %+v", issues)
			}
			wantAction := fmt.Sprintf("%d of %d cards", tc.wantRewrites, len(tc.positions))
			if !strings.Contains(issues[0].FixAction, wantAction) {
				t.Errorf("FixAction %q should mention %q", issues[0].FixAction, wantAction)
			}

			fixed, err := service.Fix(report)
			if err != nil {
				t.Fatalf("Fix failed: %v", err)
			}
			if fixed.Summary.Fixed != 1 || fixed.Summary.FixFailed != 0 {
				t.Errorf("Summary = %+v, want 1 fixed", fixed.Summary)
			}

			after := backlogCards(t, service)
			var gotOrder, gotKeys []string
			rewrites := 0
			for _, c := range after {
				gotOrder = append(gotOrder, c.ID)
				gotKeys = append(gotKeys, c.Position)
				if c.Position != before[c.ID] {
					rewrites++
				}
				if c.UpdatedAtMillis != 1700000000000 {
					t.Errorf("card %s UpdatedAtMillis changed to %d", c.ID, c.UpdatedAtMillis)
				}
			}
			if strings.Join(gotOrder, " ") != strings.Join(wantOrder, " ") {
				t.Errorf("order after fix = %v, want %v", gotOrder, wantOrder)
			}
			if rewrites != tc.wantRewrites {
				t.Errorf("rewrote %d cards, want %d", rewrites, tc.wantRewrites)
			}
			if tc.wantKeys != nil && strings.Join(gotKeys, " ") != strings.Join(tc.wantKeys, " ") {
				t.Errorf("keys after fix = %v, want %v", gotKeys, tc.wantKeys)
			}

			again, err := service.Diagnose("")
			if err != nil {
				t.Fatalf("Diagnose failed: %v", err)
			}
			if issues := positionIssues(again); len(issues) != 0 {
				t.Errorf("expected no position issues after fix, got %+v", issues)
			}
		})
	}
}
