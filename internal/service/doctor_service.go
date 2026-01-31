package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/version"
)

// IssueSeverity indicates how critical an issue is.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// Issue codes for diagnostic results.
const (
	// Priority 1: Critical data integrity (errors)
	CodeMalformedBoardConfig = "MALFORMED_BOARD_CONFIG"
	CodeMalformedCard        = "MALFORMED_CARD"
	CodeMissingCardFile      = "MISSING_CARD_FILE"
	CodeOrphanedCard         = "ORPHANED_CARD"
	CodeDuplicateCardID      = "DUPLICATE_CARD_ID"

	// Priority 2: Config issues (warnings)
	CodeSchemaOutdated      = "SCHEMA_OUTDATED"
	CodeInvalidDefaultCol   = "INVALID_DEFAULT_COLUMN"
	CodeInvalidCardDisplay  = "INVALID_CARD_DISPLAY"
	CodeInvalidLinkRule     = "INVALID_LINK_RULE"
	CodeInvalidPatternHook  = "INVALID_PATTERN_HOOK"
	CodeMissingHookFile     = "MISSING_HOOK_FILE"

	// Priority 3: Referential integrity (warnings)
	CodeInvalidParentRef = "INVALID_PARENT_REF"

	// Priority 4: Global config (warnings)
	CodeMalformedGlobalConfig = "MALFORMED_GLOBAL_CONFIG"
	CodeGlobalSchemaOutdated  = "GLOBAL_SCHEMA_OUTDATED"
)

// Issue represents a single diagnostic finding.
type Issue struct {
	Severity   IssueSeverity     `json:"severity"`
	Code       string            `json:"code"`
	Board      string            `json:"board,omitempty"`
	CardID     string            `json:"card_id,omitempty"`
	Message    string            `json:"message"`
	Fixable    bool              `json:"fixable"`
	FixAction  string            `json:"fix_action,omitempty"`
	FixError   string            `json:"fix_error,omitempty"`   // Populated if fix was attempted but failed
	FixContext map[string]string `json:"fix_context,omitempty"` // Structured data for fix logic
}

// BoardDiagnostic contains stats for a single board.
type BoardDiagnostic struct {
	Name           string `json:"name"`
	CardFiles      int    `json:"card_files"`
	CardsReferenced int   `json:"cards_referenced"`
	Columns        int    `json:"columns"`
}

// ReportSummary summarizes the diagnostic results.
type ReportSummary struct {
	Errors      int `json:"errors"`
	Warnings    int `json:"warnings"`
	Fixed       int `json:"fixed"`
	FixFailed   int `json:"fix_failed,omitempty"`
}

// DiagnosticReport contains all diagnostic results.
type DiagnosticReport struct {
	Boards  []BoardDiagnostic `json:"boards"`
	Issues  []Issue           `json:"issues"`
	Summary ReportSummary     `json:"summary"`
}

// HasErrors returns true if there are any error-level issues.
func (r *DiagnosticReport) HasErrors() bool {
	return r.Summary.Errors > 0
}

// DoctorService validates Kan data for consistency issues.
type DoctorService struct {
	paths *config.Paths
}

// NewDoctorService creates a new diagnostic service.
func NewDoctorService(paths *config.Paths) *DoctorService {
	return &DoctorService{paths: paths}
}

// Diagnose analyzes all boards (or a specific board) for issues.
// If boardName is empty, all boards are checked.
func (s *DoctorService) Diagnose(boardName string) (*DiagnosticReport, error) {
	report := &DiagnosticReport{
		Boards: []BoardDiagnostic{},
		Issues: []Issue{},
	}

	// Check global config
	s.checkGlobalConfig(report)

	// Get boards to check
	boards, err := listBoards(s.paths)
	if err != nil {
		return nil, fmt.Errorf("failed to list boards: %w", err)
	}

	for _, name := range boards {
		if boardName != "" && name != boardName {
			continue
		}
		s.checkBoard(report, name)
	}

	// Calculate summary
	for _, issue := range report.Issues {
		if issue.Severity == SeverityError {
			report.Summary.Errors++
		} else {
			report.Summary.Warnings++
		}
	}

	return report, nil
}

// Fix applies automatic fixes for issues that have deterministic solutions.
// Returns a new report showing remaining issues and what was fixed.
func (s *DoctorService) Fix(report *DiagnosticReport) (*DiagnosticReport, error) {
	fixed := 0
	fixFailed := 0
	remaining := []Issue{}

	for _, issue := range report.Issues {
		if !issue.Fixable {
			remaining = append(remaining, issue)
			continue
		}

		var err error
		switch issue.Code {
		case CodeMissingCardFile:
			err = s.fixMissingCardFile(issue.Board, issue.CardID)
		case CodeOrphanedCard:
			err = s.fixOrphanedCard(issue.Board, issue.CardID)
		case CodeDuplicateCardID:
			err = s.fixDuplicateCardID(issue.Board, issue.CardID)
		case CodeInvalidDefaultCol:
			err = s.fixInvalidDefaultColumn(issue.Board)
		case CodeInvalidCardDisplay:
			err = s.fixInvalidCardDisplay(issue.Board, issue.FixContext)
		case CodeInvalidParentRef:
			err = s.fixInvalidParentRef(issue.Board, issue.CardID)
		default:
			remaining = append(remaining, issue)
			continue
		}

		if err != nil {
			// If fix failed, keep the issue with error recorded
			issue.FixError = err.Error()
			remaining = append(remaining, issue)
			fixFailed++
		} else {
			fixed++
		}
	}

	// Rebuild report
	newReport := &DiagnosticReport{
		Boards: report.Boards,
		Issues: remaining,
		Summary: ReportSummary{
			Fixed:     fixed,
			FixFailed: fixFailed,
		},
	}

	for _, issue := range remaining {
		if issue.Severity == SeverityError {
			newReport.Summary.Errors++
		} else {
			newReport.Summary.Warnings++
		}
	}

	return newReport, nil
}

func (s *DoctorService) checkGlobalConfig(report *DiagnosticReport) {
	path := config.GlobalConfigPath()
	if path == "" {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return // No global config is fine
		}
		report.Issues = append(report.Issues, Issue{
			Severity:  SeverityWarning,
			Code:      CodeMalformedGlobalConfig,
			Message:   fmt.Sprintf("Cannot read global config: %v", err),
			Fixable:   false,
		})
		return
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		report.Issues = append(report.Issues, Issue{
			Severity:  SeverityWarning,
			Code:      CodeMalformedGlobalConfig,
			Message:   fmt.Sprintf("Invalid TOML in global config: %v", err),
			Fixable:   false,
		})
		return
	}

	// Check schema version
	if schema, ok := raw["kan_schema"].(string); ok {
		if schema != version.CurrentGlobalSchema() {
			report.Issues = append(report.Issues, Issue{
				Severity:  SeverityWarning,
				Code:      CodeGlobalSchemaOutdated,
				Message:   fmt.Sprintf("Global config has schema %s, current is %s", schema, version.CurrentGlobalSchema()),
				Fixable:   false,
				FixAction: "Run 'kan migrate' to upgrade",
			})
		}
	} else {
		report.Issues = append(report.Issues, Issue{
			Severity:  SeverityWarning,
			Code:      CodeGlobalSchemaOutdated,
			Message:   fmt.Sprintf("Global config missing schema version, current is %s", version.CurrentGlobalSchema()),
			Fixable:   false,
			FixAction: "Run 'kan migrate' to upgrade",
		})
	}
}

func (s *DoctorService) checkBoard(report *DiagnosticReport, boardName string) {
	diag := BoardDiagnostic{Name: boardName}

	// Load board config
	configPath := s.paths.BoardConfigPath(boardName)
	configData, err := os.ReadFile(configPath)
	if err != nil {
		report.Issues = append(report.Issues, Issue{
			Severity: SeverityError,
			Code:     CodeMalformedBoardConfig,
			Board:    boardName,
			Message:  fmt.Sprintf("Cannot read board config: %v", err),
			Fixable:  false,
		})
		report.Boards = append(report.Boards, diag)
		return
	}

	var boardConfig model.BoardConfig
	if _, err := toml.Decode(string(configData), &boardConfig); err != nil {
		report.Issues = append(report.Issues, Issue{
			Severity: SeverityError,
			Code:     CodeMalformedBoardConfig,
			Board:    boardName,
			Message:  fmt.Sprintf("Invalid TOML: %v", err),
			Fixable:  false,
		})
		report.Boards = append(report.Boards, diag)
		return
	}

	diag.Columns = len(boardConfig.Columns)

	// Check schema version
	s.checkBoardSchema(report, boardName, &boardConfig)

	// Check default column
	s.checkDefaultColumn(report, boardName, &boardConfig)

	// Check card display config
	s.checkCardDisplay(report, boardName, &boardConfig)

	// Check link rules
	s.checkLinkRules(report, boardName, &boardConfig)

	// Check pattern hooks
	s.checkPatternHooks(report, boardName, &boardConfig)

	// Collect card IDs from columns
	referencedCards := make(map[string]string) // cardID -> first column found in
	duplicates := make(map[string][]string)    // cardID -> all columns found in

	for _, col := range boardConfig.Columns {
		for _, cardID := range col.CardIDs {
			if firstCol, exists := referencedCards[cardID]; exists {
				duplicates[cardID] = append(duplicates[cardID], col.Name)
				if len(duplicates[cardID]) == 1 {
					duplicates[cardID] = append([]string{firstCol}, duplicates[cardID]...)
				}
			} else {
				referencedCards[cardID] = col.Name
			}
		}
	}

	diag.CardsReferenced = len(referencedCards)

	// Report duplicates
	for cardID, cols := range duplicates {
		report.Issues = append(report.Issues, Issue{
			Severity:  SeverityError,
			Code:      CodeDuplicateCardID,
			Board:     boardName,
			CardID:    cardID,
			Message:   fmt.Sprintf("Card appears in multiple columns: %s", strings.Join(cols, ", ")),
			Fixable:   true,
			FixAction: fmt.Sprintf("Keep in first column (%s), remove from others", cols[0]),
		})
	}

	// Check card files
	cardsDir := s.paths.CardsDir(boardName)
	entries, err := os.ReadDir(cardsDir)
	if err != nil && !os.IsNotExist(err) {
		report.Issues = append(report.Issues, Issue{
			Severity: SeverityWarning,
			Code:     CodeMalformedBoardConfig,
			Board:    boardName,
			Message:  fmt.Sprintf("Cannot read cards directory: %v", err),
			Fixable:  false,
		})
	}

	cardFiles := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		cardID := strings.TrimSuffix(entry.Name(), ".json")
		cardFiles[cardID] = true
		diag.CardFiles++

		// Check if card file is valid
		s.checkCardFile(report, boardName, cardID)
	}

	// Check for missing card files (referenced but no file)
	for cardID := range referencedCards {
		if !cardFiles[cardID] {
			report.Issues = append(report.Issues, Issue{
				Severity:  SeverityError,
				Code:      CodeMissingCardFile,
				Board:     boardName,
				CardID:    cardID,
				Message:   fmt.Sprintf("Card referenced in column but file not found"),
				Fixable:   true,
				FixAction: "Remove reference from column",
			})
		}
	}

	// Check for orphaned cards (file exists but not in any column)
	for cardID := range cardFiles {
		if _, referenced := referencedCards[cardID]; !referenced {
			report.Issues = append(report.Issues, Issue{
				Severity:  SeverityError,
				Code:      CodeOrphanedCard,
				Board:     boardName,
				CardID:    cardID,
				Message:   "Card file exists but not in any column",
				Fixable:   true,
				FixAction: fmt.Sprintf("Add to default column (%s)", boardConfig.GetDefaultColumn()),
			})
		}
	}

	// Check parent references
	s.checkParentRefs(report, boardName, cardFiles)

	report.Boards = append(report.Boards, diag)
}

func (s *DoctorService) checkBoardSchema(report *DiagnosticReport, boardName string, cfg *model.BoardConfig) {
	if cfg.KanSchema == "" {
		report.Issues = append(report.Issues, Issue{
			Severity:  SeverityWarning,
			Code:      CodeSchemaOutdated,
			Board:     boardName,
			Message:   fmt.Sprintf("Board missing schema version, current is %s", version.CurrentBoardSchema()),
			Fixable:   false,
			FixAction: "Run 'kan migrate' to upgrade",
		})
		return
	}

	if cfg.KanSchema != version.CurrentBoardSchema() {
		report.Issues = append(report.Issues, Issue{
			Severity:  SeverityWarning,
			Code:      CodeSchemaOutdated,
			Board:     boardName,
			Message:   fmt.Sprintf("Board has schema %s, current is %s", cfg.KanSchema, version.CurrentBoardSchema()),
			Fixable:   false,
			FixAction: "Run 'kan migrate' to upgrade",
		})
	}
}

func (s *DoctorService) checkDefaultColumn(report *DiagnosticReport, boardName string, cfg *model.BoardConfig) {
	if cfg.DefaultColumn == "" {
		return // Will use first column as default
	}

	if !cfg.HasColumn(cfg.DefaultColumn) {
		fixAction := "Clear default_column (no columns exist)"
		if len(cfg.Columns) > 0 {
			fixAction = fmt.Sprintf("Reset to first column (%s)", cfg.Columns[0].Name)
		}
		report.Issues = append(report.Issues, Issue{
			Severity:  SeverityWarning,
			Code:      CodeInvalidDefaultCol,
			Board:     boardName,
			Message:   fmt.Sprintf("default_column '%s' does not exist", cfg.DefaultColumn),
			Fixable:   true,
			FixAction: fixAction,
		})
	}
}

func (s *DoctorService) checkCardDisplay(report *DiagnosticReport, boardName string, cfg *model.BoardConfig) {
	warnings := cfg.ValidateCardDisplay()
	for _, w := range warnings {
		// Determine which field type this warning is about
		fieldType := ""
		if strings.Contains(w, "type_indicator") {
			fieldType = "type_indicator"
		} else if strings.Contains(w, "badges") {
			fieldType = "badges"
		} else if strings.Contains(w, "metadata") {
			fieldType = "metadata"
		}

		issue := Issue{
			Severity:  SeverityWarning,
			Code:      CodeInvalidCardDisplay,
			Board:     boardName,
			Message:   w,
			Fixable:   true,
			FixAction: "Remove invalid reference",
		}
		if fieldType != "" {
			issue.FixContext = map[string]string{"field": fieldType}
		}
		report.Issues = append(report.Issues, issue)
	}
}

func (s *DoctorService) checkLinkRules(report *DiagnosticReport, boardName string, cfg *model.BoardConfig) {
	for _, rule := range cfg.LinkRules {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			report.Issues = append(report.Issues, Issue{
				Severity:  SeverityWarning,
				Code:      CodeInvalidLinkRule,
				Board:     boardName,
				Message:   fmt.Sprintf("Link rule '%s' has invalid regex: %v", rule.Name, err),
				Fixable:   false,
			})
		}
	}
}

func (s *DoctorService) checkPatternHooks(report *DiagnosticReport, boardName string, cfg *model.BoardConfig) {
	for _, hook := range cfg.PatternHooks {
		// Check regex
		if _, err := regexp.Compile(hook.PatternTitle); err != nil {
			report.Issues = append(report.Issues, Issue{
				Severity:  SeverityWarning,
				Code:      CodeInvalidPatternHook,
				Board:     boardName,
				Message:   fmt.Sprintf("Pattern hook '%s' has invalid regex: %v", hook.Name, err),
				Fixable:   false,
			})
		}

		// Check if command file exists (for file-based commands)
		cmd := hook.Command
		if cmd == "" {
			continue
		}

		// Expand ~ to home directory
		if strings.HasPrefix(cmd, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				cmd = filepath.Join(home, cmd[2:])
			}
		}

		// Only check if it looks like a file path (starts with / or ~/ or ./)
		if strings.HasPrefix(cmd, "/") || strings.HasPrefix(cmd, "./") {
			// Extract the first word (the executable)
			parts := strings.Fields(cmd)
			if len(parts) > 0 {
				execPath := parts[0]
				if _, err := os.Stat(execPath); os.IsNotExist(err) {
					report.Issues = append(report.Issues, Issue{
						Severity:  SeverityWarning,
						Code:      CodeMissingHookFile,
						Board:     boardName,
						Message:   fmt.Sprintf("Pattern hook '%s' references non-existent file: %s", hook.Name, execPath),
						Fixable:   false,
					})
				}
			}
		}
	}
}

func (s *DoctorService) checkCardFile(report *DiagnosticReport, boardName, cardID string) {
	cardPath := s.paths.CardPath(boardName, cardID)
	data, err := os.ReadFile(cardPath)
	if err != nil {
		report.Issues = append(report.Issues, Issue{
			Severity: SeverityError,
			Code:     CodeMalformedCard,
			Board:    boardName,
			CardID:   cardID,
			Message:  fmt.Sprintf("Cannot read card file: %v", err),
			Fixable:  false,
		})
		return
	}

	var card model.Card
	if err := json.Unmarshal(data, &card); err != nil {
		report.Issues = append(report.Issues, Issue{
			Severity: SeverityError,
			Code:     CodeMalformedCard,
			Board:    boardName,
			CardID:   cardID,
			Message:  fmt.Sprintf("Invalid JSON: %v", err),
			Fixable:  false,
		})
	}
}

func (s *DoctorService) checkParentRefs(report *DiagnosticReport, boardName string, cardFiles map[string]bool) {
	for cardID := range cardFiles {
		cardPath := s.paths.CardPath(boardName, cardID)
		data, err := os.ReadFile(cardPath)
		if err != nil {
			continue // Already reported in checkCardFile
		}

		var card model.Card
		if err := json.Unmarshal(data, &card); err != nil {
			continue // Already reported in checkCardFile
		}

		if card.Parent != "" && !cardFiles[card.Parent] {
			report.Issues = append(report.Issues, Issue{
				Severity:  SeverityWarning,
				Code:      CodeInvalidParentRef,
				Board:     boardName,
				CardID:    cardID,
				Message:   fmt.Sprintf("Parent '%s' does not exist", card.Parent),
				Fixable:   true,
				FixAction: "Clear parent field",
			})
		}
	}
}

// Fix implementations

func (s *DoctorService) fixMissingCardFile(boardName, cardID string) error {
	configPath := s.paths.BoardConfigPath(boardName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var cfg model.BoardConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return err
	}

	// Remove cardID from all columns
	for i := range cfg.Columns {
		newIDs := []string{}
		for _, id := range cfg.Columns[i].CardIDs {
			if id != cardID {
				newIDs = append(newIDs, id)
			}
		}
		cfg.Columns[i].CardIDs = newIDs
	}

	return writeBoardConfig(configPath, &cfg)
}

func (s *DoctorService) fixOrphanedCard(boardName, cardID string) error {
	configPath := s.paths.BoardConfigPath(boardName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var cfg model.BoardConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return err
	}

	// Add to default column at the top
	defaultCol := cfg.GetDefaultColumn()
	cfg.InsertCardInColumn(cardID, defaultCol, 0)

	return writeBoardConfig(configPath, &cfg)
}

func (s *DoctorService) fixDuplicateCardID(boardName, cardID string) error {
	configPath := s.paths.BoardConfigPath(boardName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var cfg model.BoardConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return err
	}

	// Keep the first occurrence, remove from all other columns
	found := false
	for i := range cfg.Columns {
		newIDs := []string{}
		for _, id := range cfg.Columns[i].CardIDs {
			if id == cardID {
				if !found {
					found = true
					newIDs = append(newIDs, id)
				}
				// Skip duplicates
			} else {
				newIDs = append(newIDs, id)
			}
		}
		cfg.Columns[i].CardIDs = newIDs
	}

	return writeBoardConfig(configPath, &cfg)
}

func (s *DoctorService) fixInvalidDefaultColumn(boardName string) error {
	configPath := s.paths.BoardConfigPath(boardName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var cfg model.BoardConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return err
	}

	if len(cfg.Columns) > 0 {
		cfg.DefaultColumn = cfg.Columns[0].Name
	} else {
		cfg.DefaultColumn = ""
	}

	return writeBoardConfig(configPath, &cfg)
}

func (s *DoctorService) fixInvalidCardDisplay(boardName string, fixContext map[string]string) error {
	configPath := s.paths.BoardConfigPath(boardName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var cfg model.BoardConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return err
	}

	// Use fix context to determine what to fix
	fieldType := ""
	if fixContext != nil {
		fieldType = fixContext["field"]
	}

	switch fieldType {
	case "type_indicator":
		cfg.CardDisplay.TypeIndicator = ""
	case "badges":
		cfg.CardDisplay.Badges = filterValidFields(cfg.CardDisplay.Badges, cfg.CustomFields)
	case "metadata":
		cfg.CardDisplay.Metadata = filterValidFields(cfg.CardDisplay.Metadata, cfg.CustomFields)
	default:
		return fmt.Errorf("unknown card_display field type: %q", fieldType)
	}

	return writeBoardConfig(configPath, &cfg)
}

func filterValidFields(fields []string, customFields map[string]model.CustomFieldSchema) []string {
	result := []string{}
	for _, f := range fields {
		if _, exists := customFields[f]; exists {
			result = append(result, f)
		}
	}
	return result
}

func (s *DoctorService) fixInvalidParentRef(boardName, cardID string) error {
	cardPath := s.paths.CardPath(boardName, cardID)
	data, err := os.ReadFile(cardPath)
	if err != nil {
		return err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	delete(raw, "parent")

	return writeJSONMap(cardPath, raw)
}

