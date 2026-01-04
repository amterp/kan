package editor

import (
	"os"
	"os/exec"

	"github.com/amterp/kan/internal/model"
)

// Editor handles editor resolution and invocation.
type Editor struct {
	globalConfig *model.GlobalConfig
}

// NewEditor creates a new Editor.
func NewEditor(globalConfig *model.GlobalConfig) *Editor {
	return &Editor{globalConfig: globalConfig}
}

// Resolve returns the editor command to use.
// Order: global config > $EDITOR > vim
func (e *Editor) Resolve() string {
	// 1. Global config
	if e.globalConfig != nil && e.globalConfig.Editor != "" {
		return e.globalConfig.Editor
	}

	// 2. Environment variable
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// 3. Default
	return "vim"
}

// Edit opens the editor with the given content and returns the edited content.
func (e *Editor) Edit(content string) (string, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "kan-edit-*.md")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write content
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// Open editor
	editor := e.Resolve()
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Read back content
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}

	return string(edited), nil
}
