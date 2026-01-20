package version

import (
	"fmt"
)

// SchemaVersionError indicates a schema version problem during file read/write.
type SchemaVersionError struct {
	FileType    string // "card", "board", "global"
	FilePath    string // Path to the problematic file
	Found       string // What was found (e.g., "missing", "2", "board/2")
	Expected    string // What was expected (e.g., "1", "board/1")
	MinRequired string // Minimum Kan version required (if upgrade needed)
}

func (e *SchemaVersionError) Error() string {
	if e.MinRequired != "" {
		return fmt.Sprintf(
			"%s schema version %s requires Kan >= %s (file: %s, found: %s, supports up to: %s)",
			e.FileType, e.Found, e.MinRequired, e.FilePath, e.Found, e.Expected,
		)
	}
	if e.Found == "missing" {
		return fmt.Sprintf(
			"%s has no schema version (file: %s). Run 'kan migrate' to upgrade.",
			e.FileType, e.FilePath,
		)
	}
	return fmt.Sprintf(
		"%s has invalid schema version: found %s, expected %s (file: %s)",
		e.FileType, e.Found, e.Expected, e.FilePath,
	)
}

// MissingCardVersion creates an error for a card file missing the _v field.
func MissingCardVersion(path string) error {
	return &SchemaVersionError{
		FileType: "card",
		FilePath: path,
		Found:    "missing",
		Expected: fmt.Sprintf("%d", CurrentCardVersion),
	}
}

// InvalidCardVersion creates an error for a card with an unsupported version.
func InvalidCardVersion(path string, found, expected int) error {
	e := &SchemaVersionError{
		FileType: "card",
		FilePath: path,
		Found:    fmt.Sprintf("%d", found),
		Expected: fmt.Sprintf("%d", expected),
	}
	// If the found version is newer, look up the min required Kan version
	if found > expected {
		key := fmt.Sprintf("card/%d", found)
		if minKan, ok := MinKanVersion[key]; ok {
			e.MinRequired = minKan
		} else {
			e.MinRequired = "a newer version"
		}
	}
	return e
}

// MissingBoardSchema creates an error for a board config missing kan_schema.
func MissingBoardSchema(path string) error {
	return &SchemaVersionError{
		FileType: "board config",
		FilePath: path,
		Found:    "missing",
		Expected: CurrentBoardSchema(),
	}
}

// InvalidBoardSchema creates an error for a board with an unsupported schema.
func InvalidBoardSchema(path, found string) error {
	e := &SchemaVersionError{
		FileType: "board config",
		FilePath: path,
		Found:    found,
		Expected: CurrentBoardSchema(),
	}
	// Check if it's a future version
	if v, err := ParseBoardVersion(found); err == nil && v > CurrentBoardVersion {
		if minKan, ok := MinKanVersion[found]; ok {
			e.MinRequired = minKan
		} else {
			e.MinRequired = "a newer version"
		}
	}
	return e
}

// MissingGlobalSchema creates an error for a global config missing kan_schema.
func MissingGlobalSchema(path string) error {
	return &SchemaVersionError{
		FileType: "global config",
		FilePath: path,
		Found:    "missing",
		Expected: CurrentGlobalSchema(),
	}
}

// InvalidGlobalSchema creates an error for a global config with unsupported schema.
func InvalidGlobalSchema(path, found string) error {
	e := &SchemaVersionError{
		FileType: "global config",
		FilePath: path,
		Found:    found,
		Expected: CurrentGlobalSchema(),
	}
	// Check if it's a future version
	if v, err := ParseGlobalVersion(found); err == nil && v > CurrentGlobalVersion {
		if minKan, ok := MinKanVersion[found]; ok {
			e.MinRequired = minKan
		} else {
			e.MinRequired = "a newer version"
		}
	}
	return e
}

// MissingProjectSchema creates an error for a project config missing kan_schema.
func MissingProjectSchema(path string) error {
	return &SchemaVersionError{
		FileType: "project config",
		FilePath: path,
		Found:    "missing",
		Expected: CurrentProjectSchema(),
	}
}

// InvalidProjectSchema creates an error for a project config with unsupported schema.
func InvalidProjectSchema(path, found string) error {
	e := &SchemaVersionError{
		FileType: "project config",
		FilePath: path,
		Found:    found,
		Expected: CurrentProjectSchema(),
	}
	// Check if it's a future version
	if v, err := ParseProjectVersion(found); err == nil && v > CurrentProjectVersion {
		if minKan, ok := MinKanVersion[found]; ok {
			e.MinRequired = minKan
		} else {
			e.MinRequired = "a newer version"
		}
	}
	return e
}
