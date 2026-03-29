package discovery

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
)

// ErrStaleGlobalConfig indicates the global config references a project path
// but the kan data directory doesn't exist. This can happen if the user
// manually deletes the .kan/ directory.
var ErrStaleGlobalConfig = errors.New("stale global config entry")

// WorktreeResolver detects git worktrees and resolves the main worktree root.
type WorktreeResolver interface {
	IsWorktree() bool
	GetMainWorktreeRoot() (string, error)
}

// Result contains the discovered project root and data location.
type Result struct {
	ProjectRoot          string // Absolute path to project root
	DataLocation         string // Relative path for kan data (empty = default .kan/)
	WasRegistered        bool   // Whether this project was found in global config
	ResolvedFromWorktree bool   // True if redirected from a worktree to main
	OriginalWorktreeRoot string // The worktree path we redirected from (empty if not redirected)
}

// DiscoverProject finds the project root by walking up from cwd.
// Priority:
// 1. Directory that's a key in global config -> use configured DataLocation
// 2. Directory containing .kan/ -> use as self-discoverable default
//
// Returns nil if no project found (not initialized).
// Returns error if global config references a path but data is missing.
func DiscoverProject(globalCfg *model.GlobalConfig) (*Result, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	return DiscoverProjectFrom(cwd, globalCfg)
}

// DiscoverProjectFrom finds the project root starting from a given directory.
func DiscoverProjectFrom(startDir string, globalCfg *model.GlobalConfig) (*Result, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	dir := absStart
	for {
		// Check 1: Is this directory a key in global config?
		if globalCfg != nil {
			if repoCfg := globalCfg.GetRepoConfig(dir); repoCfg != nil {
				dataLocation := repoCfg.DataLocation
				if dataLocation == "" {
					dataLocation = config.DefaultKanDir
				}
				dataPath := filepath.Join(dir, dataLocation, config.BoardsDir)
				if _, err := os.Stat(dataPath); err == nil {
					return &Result{
						ProjectRoot:   dir,
						DataLocation:  repoCfg.DataLocation,
						WasRegistered: true,
					}, nil
				}
				// Global config says this path exists but data is missing
				return nil, fmt.Errorf("%w: global config references %s but kan data not found at %s",
					ErrStaleGlobalConfig, dir, filepath.Join(dir, dataLocation))
			}
		}

		// Check 2: Does .kan/ exist here (self-discoverable default)?
		kanBoardsDir := filepath.Join(dir, config.DefaultKanDir, config.BoardsDir)
		if _, err := os.Stat(kanBoardsDir); err == nil {
			return &Result{
				ProjectRoot:   dir,
				DataLocation:  "",
				WasRegistered: false,
			}, nil
		}

		// Move up to parent
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, no project found
			return nil, nil
		}
		dir = parent
	}
}

// ResolveWorktree checks if the discovered project is in a git worktree and
// redirects to the main worktree's project if so. This makes all worktrees
// share the main worktree's board by default.
//
// The isIndependent callback checks whether the discovered project has opted
// out of worktree sharing (via worktree_independent in project config).
// Pass nil to skip the independence check.
//
// Falls back to the original result if: not in a worktree, the project opted
// out, the main worktree has no project, or any error occurs.
func ResolveWorktree(
	result *Result,
	resolver WorktreeResolver,
	globalCfg *model.GlobalConfig,
	isIndependent func(projectRoot, dataLocation string) bool,
) (*Result, error) {
	if result == nil || resolver == nil || !resolver.IsWorktree() {
		return result, nil
	}

	// Check if this project has opted out of worktree sharing
	if isIndependent != nil && isIndependent(result.ProjectRoot, result.DataLocation) {
		return result, nil
	}

	mainRoot, err := resolver.GetMainWorktreeRoot()
	if err != nil {
		// Can't determine main worktree - fall back to local
		return result, nil
	}

	// Re-discover from the main worktree root
	mainResult, err := DiscoverProjectFrom(mainRoot, globalCfg)
	if err != nil || mainResult == nil {
		// Main worktree has no project - fall back to local
		return result, nil
	}

	mainResult.ResolvedFromWorktree = true
	mainResult.OriginalWorktreeRoot = result.ProjectRoot
	return mainResult, nil
}
