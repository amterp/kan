package service

import (
	"bytes"
	"context"
	"errors"
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

// hookWaitDelay is the grace period allowed for a killed hook's output to be flushed
// before its pipes are force-closed.
const hookWaitDelay = 2 * time.Second

// HookResult contains the result of executing a hook.
type HookResult struct {
	HookName string
	Command  string
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
	// Determine timeout
	timeout := hook.Timeout
	if timeout <= 0 {
		timeout = DefaultHookTimeout
	}

	// Expand ~ in command path
	command := expandTilde(hook.Command)

	// Record the expanded command: when an exec fails, the path that was actually
	// attempted is what the operator needs, not the tilde form from the config.
	result := &HookResult{
		HookName: hook.Name,
		Command:  command,
	}

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

	// Killing the hook on timeout doesn't kill any grandchildren it spawned, and those
	// inherit the output pipes - so Run would otherwise block reading from them long
	// after the deadline, making the timeout unenforceable. WaitDelay caps that wait.
	cmd.WaitDelay = hookWaitDelay

	// Run the command
	start := time.Now()
	err := cmd.Run()
	result.Duration = time.Since(start)

	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err != nil {
		result.Error = err
		result.Success = false

		switch {
		// Checked before *exec.ExitError: killing the process on deadline still yields
		// an ExitError, so testing that first would report a bare "signal: killed".
		case ctx.Err() == context.DeadlineExceeded:
			result.Error = fmt.Errorf("hook timed out after %ds", timeout)
			result.ExitCode = -1

		// ErrWaitDelay means the hook itself exited successfully but something it
		// spawned outlived it still holding the output pipes. That's a legitimate
		// fire-and-forget hook, not a failure - at most we dropped a tail of output.
		case errors.Is(err, exec.ErrWaitDelay):
			result.Success = true
			result.ExitCode = 0
			result.Error = nil

		default:
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = -1
			}
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	return result
}

// exitCodeCommandNotFound is the shell convention for "command not found".
const exitCodeCommandNotFound = 127

// HookPathHint explains the most common way a hook fails. Exit 127 and a bare ENOENT
// are opaque unless you already know the convention, and the leap from there to "my
// background service has a different PATH than my shell" is exactly what costs hours.
const HookPathHint = "The command or its interpreter was not found. " +
	"Hooks inherit the environment of whatever started Kan, and a background service " +
	"(launchd, systemd) gets a minimal PATH - set PATH explicitly in the service definition."

// CommandNotFound reports whether the hook failed because its command - or the
// interpreter named in its shebang - could not be found. This is the signature of a
// PATH problem, which is easy to hit when Kan runs as a background service and
// inherits a minimal PATH rather than a shell's.
func (r *HookResult) CommandNotFound() bool {
	if r.Success {
		return false
	}
	if r.ExitCode == exitCodeCommandNotFound {
		return true
	}
	return errors.Is(r.Error, exec.ErrNotFound) || errors.Is(r.Error, os.ErrNotExist)
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
