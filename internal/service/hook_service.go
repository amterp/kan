package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/amterp/kan/internal/model"
)

// DefaultHookTimeout is the default timeout for hook execution in seconds.
const DefaultHookTimeout = 30

// HookResult contains the result of executing a hook.
type HookResult struct {
	HookName string
	Success  bool
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Error    error
}

// HookService handles pattern hook execution.
type HookService struct {
	projectRoot string
}

// NewHookService creates a new hook service.
func NewHookService(projectRoot string) *HookService {
	return &HookService{
		projectRoot: projectRoot,
	}
}

// FindMatchingHooks returns all hooks whose pattern matches the given card title.
func (s *HookService) FindMatchingHooks(hooks []model.PatternHook, title string) []model.PatternHook {
	var matching []model.PatternHook
	for _, hook := range hooks {
		re, err := regexp.Compile(hook.PatternTitle)
		if err != nil {
			// Skip invalid patterns (should have been caught by validation)
			continue
		}
		if re.MatchString(title) {
			matching = append(matching, hook)
		}
	}
	return matching
}

// ExecuteHook runs a hook command with the card ID and board name as arguments.
// Returns the hook result including stdout, stderr, exit code, and any error.
func (s *HookService) ExecuteHook(hook model.PatternHook, cardID, boardName string) *HookResult {
	result := &HookResult{
		HookName: hook.Name,
	}

	// Determine timeout
	timeout := hook.Timeout
	if timeout <= 0 {
		timeout = DefaultHookTimeout
	}

	// Expand ~ in command path
	command := expandTilde(hook.Command)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctx, command, cardID, boardName)

	// Set working directory to project root
	cmd.Dir = s.projectRoot

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	start := time.Now()
	err := cmd.Run()
	result.Duration = time.Since(start)

	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		result.Error = err
		result.Success = false

		// Try to get exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("hook timed out after %ds", timeout)
			result.ExitCode = -1
		} else {
			result.ExitCode = -1
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	return result
}

// ExecuteHooks runs all matching hooks sequentially and returns their results.
func (s *HookService) ExecuteHooks(hooks []model.PatternHook, cardID, boardName string) []*HookResult {
	var results []*HookResult
	for _, hook := range hooks {
		result := s.ExecuteHook(hook, cardID, boardName)
		results = append(results, result)
	}
	return results
}

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
