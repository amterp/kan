package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
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

	if err := copyDir(fixtureDir, tempDir); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to copy test fixtures: %v", err)
	}

	paths := config.NewPaths(tempDir, "")
	service := NewDoctorService(paths)

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

	// Verify the card was added to the default column
	configPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	configStr := string(data)
	if !strings.Contains(configStr, "card-orphan") {
		t.Error("Fixed config should contain card-orphan in a column")
	}
}

func TestDoctorService_MissingCard(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "missing-card")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", report.Summary.Errors)
	}

	// Find the missing card issue
	found := false
	for _, issue := range report.Issues {
		if issue.Code == CodeMissingCardFile && issue.CardID == "card-missing" {
			found = true
			if !issue.Fixable {
				t.Error("Missing card issue should be fixable")
			}
		}
	}
	if !found {
		t.Error("Expected MISSING_CARD_FILE issue for card-missing")
	}
}

func TestDoctorService_MissingCard_Fix(t *testing.T) {
	service, tempDir, cleanup := setupDoctorTest(t, "missing-card")
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

	// Verify the missing card reference was removed
	configPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	configStr := string(data)
	if strings.Contains(configStr, "card-missing") {
		t.Error("Fixed config should not contain card-missing reference")
	}
}

func TestDoctorService_DuplicateCard(t *testing.T) {
	service, _, cleanup := setupDoctorTest(t, "duplicate-card")
	defer cleanup()

	report, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}

	if report.Summary.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", report.Summary.Errors)
	}

	// Find the duplicate card issue
	found := false
	for _, issue := range report.Issues {
		if issue.Code == CodeDuplicateCardID && issue.CardID == "card-1" {
			found = true
			if !issue.Fixable {
				t.Error("Duplicate card issue should be fixable")
			}
		}
	}
	if !found {
		t.Error("Expected DUPLICATE_CARD_ID issue for card-1")
	}
}

func TestDoctorService_DuplicateCard_Fix(t *testing.T) {
	service, tempDir, cleanup := setupDoctorTest(t, "duplicate-card")
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

	// Re-run diagnosis to verify fix
	newReport, err := service.Diagnose("")
	if err != nil {
		t.Fatalf("Second diagnose failed: %v", err)
	}

	if newReport.Summary.Errors != 0 {
		t.Errorf("Expected 0 errors after fix, got %d", newReport.Summary.Errors)
	}

	// Verify the card is only in one column now (the first one: backlog)
	configPath := filepath.Join(tempDir, ".kan", "boards", "main", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Count occurrences of card-1
	count := countOccurrences(string(data), "card-1")
	if count != 1 {
		t.Errorf("Expected card-1 to appear once in config, appeared %d times", count)
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
	service, _, cleanup := setupDoctorTest(t, "missing-card")
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

// Helper functions

func countOccurrences(s, substr string) int {
	return strings.Count(s, substr)
}
