package prompt

import (
	"github.com/charmbracelet/huh"
)

// HuhPrompter implements Prompter using the charmbracelet/huh library.
type HuhPrompter struct{}

// NewHuhPrompter creates a new huh-based prompter.
func NewHuhPrompter() *HuhPrompter {
	return &HuhPrompter{}
}

func (p *HuhPrompter) Select(title string, options []string) (string, error) {
	var result string

	opts := make([]huh.Option[string], len(options))
	for i, opt := range options {
		opts[i] = huh.NewOption(opt, opt)
	}

	err := huh.NewSelect[string]().
		Title(title).
		Options(opts...).
		Value(&result).
		Run()

	return result, err
}

func (p *HuhPrompter) Input(title string, defaultValue string) (string, error) {
	var result string

	input := huh.NewInput().
		Title(title).
		Value(&result)

	if defaultValue != "" {
		result = defaultValue
	}

	err := input.Run()
	return result, err
}

func (p *HuhPrompter) Confirm(title string, defaultValue bool) (bool, error) {
	result := defaultValue

	err := huh.NewConfirm().
		Title(title).
		Value(&result).
		Run()

	return result, err
}

func (p *HuhPrompter) MultiSelect(title string, options []string) ([]string, error) {
	var result []string

	opts := make([]huh.Option[string], len(options))
	for i, opt := range options {
		opts[i] = huh.NewOption(opt, opt)
	}

	err := huh.NewMultiSelect[string]().
		Title(title).
		Options(opts...).
		Value(&result).
		Run()

	return result, err
}
