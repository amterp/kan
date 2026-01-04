package prompt

import "errors"

// ErrNonInteractive is returned when prompting in non-interactive mode.
var ErrNonInteractive = errors.New("cannot prompt in non-interactive mode")

// Prompter defines the interface for interactive user prompts.
type Prompter interface {
	// Select presents options and returns the selected value.
	Select(title string, options []string) (string, error)

	// Input prompts for text input.
	Input(title string, defaultValue string) (string, error)

	// Confirm prompts for yes/no.
	Confirm(title string, defaultValue bool) (bool, error)

	// MultiSelect allows selecting multiple options.
	MultiSelect(title string, options []string) ([]string, error)
}

// NoopPrompter returns errors for all prompts (non-interactive mode).
type NoopPrompter struct{}

func (p *NoopPrompter) Select(title string, options []string) (string, error) {
	return "", ErrNonInteractive
}

func (p *NoopPrompter) Input(title string, defaultValue string) (string, error) {
	return "", ErrNonInteractive
}

func (p *NoopPrompter) Confirm(title string, defaultValue bool) (bool, error) {
	return false, ErrNonInteractive
}

func (p *NoopPrompter) MultiSelect(title string, options []string) ([]string, error) {
	return nil, ErrNonInteractive
}
