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

// Result contains the discovered project root and data location.
type Result struct {
	ProjectRoot   string // Absolute path to project root
	DataLocation  string // Relative path for kan data (empty = default .kan/)
	WasRegistered bool   // Whether this project was found in global config
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
