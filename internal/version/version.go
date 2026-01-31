package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Current schema versions - bump these when making breaking changes.
//
// CHECKLIST when bumping a version:
//  1. Update the constant below
//  2. Add entry to MinKanVersion map (tested by TestMinKanVersionCompleteness)
//  3. Add testdata/migrations/vN/ fixtures (tested by TestMigrationFixturesComplete)
//  4. Add migration tests in migrate_service_test.go
//  5. Update COMPAT.md with migration details
const (
	CurrentCardVersion    = 1
	CurrentBoardVersion   = 3
	CurrentGlobalVersion  = 1
	CurrentProjectVersion = 1
)

// Schema type prefixes for config files.
const (
	BoardSchemaPrefix   = "board/"
	GlobalSchemaPrefix  = "global/"
	ProjectSchemaPrefix = "project/"
)

// MinKanVersion maps schema identifiers to the minimum Kan version required.
// Used to provide helpful upgrade messages when encountering newer schemas.
var MinKanVersion = map[string]string{
	"card/1":    "0.1.0",
	"board/1":   "0.1.0",
	"board/2":   "0.2.0",
	"board/3":   "0.4.0",
	"global/1":  "0.1.0",
	"project/1": "0.3.0",
}

// FormatBoardSchema creates a board schema string from a version number.
// Example: FormatBoardSchema(1) returns "board/1"
func FormatBoardSchema(v int) string {
	return fmt.Sprintf("%s%d", BoardSchemaPrefix, v)
}

// FormatGlobalSchema creates a global schema string from a version number.
// Example: FormatGlobalSchema(1) returns "global/1"
func FormatGlobalSchema(v int) string {
	return fmt.Sprintf("%s%d", GlobalSchemaPrefix, v)
}

// ParseBoardVersion extracts the version number from a board schema string.
// Returns an error if the format is invalid.
func ParseBoardVersion(schema string) (int, error) {
	return parseSchemaVersion(schema, BoardSchemaPrefix, "board")
}

// ParseGlobalVersion extracts the version number from a global schema string.
// Returns an error if the format is invalid.
func ParseGlobalVersion(schema string) (int, error) {
	return parseSchemaVersion(schema, GlobalSchemaPrefix, "global")
}

func parseSchemaVersion(schema, prefix, schemaType string) (int, error) {
	if !strings.HasPrefix(schema, prefix) {
		return 0, fmt.Errorf("invalid %s schema format: %q (expected %sN)", schemaType, schema, prefix)
	}
	versionStr := strings.TrimPrefix(schema, prefix)
	v, err := strconv.Atoi(versionStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s schema version: %q", schemaType, versionStr)
	}
	if v < 1 {
		return 0, fmt.Errorf("invalid %s schema version: %d (must be >= 1)", schemaType, v)
	}
	return v, nil
}

// CurrentBoardSchema returns the current board schema string.
func CurrentBoardSchema() string {
	return FormatBoardSchema(CurrentBoardVersion)
}

// CurrentGlobalSchema returns the current global schema string.
func CurrentGlobalSchema() string {
	return FormatGlobalSchema(CurrentGlobalVersion)
}

// FormatProjectSchema creates a project schema string from a version number.
// Example: FormatProjectSchema(1) returns "project/1"
func FormatProjectSchema(v int) string {
	return fmt.Sprintf("%s%d", ProjectSchemaPrefix, v)
}

// ParseProjectVersion extracts the version number from a project schema string.
// Returns an error if the format is invalid.
func ParseProjectVersion(schema string) (int, error) {
	return parseSchemaVersion(schema, ProjectSchemaPrefix, "project")
}

// CurrentProjectSchema returns the current project schema string.
func CurrentProjectSchema() string {
	return FormatProjectSchema(CurrentProjectVersion)
}
