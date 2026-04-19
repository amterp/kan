package errors

import (
	"errors"
	"fmt"
	"strings"
)

// Standard sentinel errors for type checking
var (
	ErrNotFound       = errors.New("not found")
	ErrAlreadyExists  = errors.New("already exists")
	ErrNotInitialized = errors.New("not initialized")
	ErrInvalidInput   = errors.New("invalid input")
	ErrAmbiguous      = errors.New("ambiguous match")
)

// NotFoundError indicates a resource doesn't exist.
type NotFoundError struct {
	Resource string // "card", "board", "config"
	ID       string // The identifier that wasn't found
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// AlreadyExistsError indicates a resource already exists.
type AlreadyExistsError struct {
	Resource string
	ID       string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s already exists: %s", e.Resource, e.ID)
}

func (e *AlreadyExistsError) Unwrap() error {
	return ErrAlreadyExists
}

// ValidationError indicates invalid user input.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
	}
	return e.Message
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidInput
}

// AmbiguousMatch is one candidate in an AmbiguousCardError list.
type AmbiguousMatch struct {
	Alias string
	ID    string
}

// AmbiguousCardError indicates a fuzzy lookup matched multiple cards and
// the user must disambiguate. Matches contains the full list (pre-sorted);
// DisplayLimit caps how many entries Error() renders. Storing the full list
// lets callers like cross-board aggregation reuse the data without losing
// truncated matches.
type AmbiguousCardError struct {
	Input        string
	Matches      []AmbiguousMatch
	DisplayLimit int
}

func (e *AmbiguousCardError) Error() string {
	shown := e.Matches
	total := len(e.Matches)
	if e.DisplayLimit > 0 && total > e.DisplayLimit {
		shown = e.Matches[:e.DisplayLimit]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "multiple cards match %q:", e.Input)
	maxAlias := 0
	for _, m := range shown {
		if len(m.Alias) > maxAlias {
			maxAlias = len(m.Alias)
		}
	}
	for _, m := range shown {
		fmt.Fprintf(&b, "\n  %-*s  (%s)", maxAlias, m.Alias, m.ID)
	}
	if total > len(shown) {
		fmt.Fprintf(&b, "\n(showing %d of %d)", len(shown), total)
	}
	return b.String()
}

func (e *AmbiguousCardError) Unwrap() error {
	return ErrAmbiguous
}

// NotInitializedError indicates Kan isn't set up in the repo.
type NotInitializedError struct {
	Path string
}

func (e *NotInitializedError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("kan not initialized in %s (run 'kan init')", e.Path)
	}
	return "kan not initialized (run 'kan init')"
}

func (e *NotInitializedError) Unwrap() error {
	return ErrNotInitialized
}

// Helper constructors for common cases

func CardNotFound(idOrAlias string) error {
	return &NotFoundError{Resource: "card", ID: idOrAlias}
}

func BoardNotFound(name string) error {
	return &NotFoundError{Resource: "board", ID: name}
}

func ColumnNotFound(name, board string) error {
	return &NotFoundError{Resource: "column", ID: fmt.Sprintf("%s (in board %s)", name, board)}
}

func CommentNotFound(id string) error {
	return &NotFoundError{Resource: "comment", ID: id}
}

// NewAmbiguousCardError builds an AmbiguousCardError. The full match list is
// stored on the error; displayLimit controls how many are shown by Error()
// (Error() renders "(showing N of M)" when truncation happens). Pass 0 to
// disable the display cap.
func NewAmbiguousCardError(input string, matches []AmbiguousMatch, displayLimit int) error {
	return &AmbiguousCardError{Input: input, Matches: matches, DisplayLimit: displayLimit}
}

func BoardAlreadyExists(name string) error {
	return &AlreadyExistsError{Resource: "board", ID: name}
}

func ColumnAlreadyExists(name, board string) error {
	return &AlreadyExistsError{Resource: "column", ID: fmt.Sprintf("%s (in board %s)", name, board)}
}

func InvalidField(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

func ColumnLimitExceeded(columnName string, limit int) error {
	return &ValidationError{
		Message: fmt.Sprintf("column %q is full (limit: %d)", columnName, limit),
	}
}

// IsNotFound checks if an error is a not-found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is an already-exists error.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsAmbiguous reports whether err indicates an ambiguous fuzzy match.
func IsAmbiguous(err error) bool {
	return errors.Is(err, ErrAmbiguous)
}
