package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/id"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

const defaultBoardName = "main"

// InitService handles project initialization.
type InitService struct {
	globalStore store.GlobalStore
}

// NewInitService creates a new init service.
func NewInitService(globalStore store.GlobalStore) *InitService {
	return &InitService{
		globalStore: globalStore,
	}
}

// Initialize initializes Kan in the current directory.
// If customLocation is empty, uses the default .kan directory.
// If boardName is empty, uses "main".
// If customColumns is empty, uses default columns.
func (s *InitService) Initialize(customLocation, boardName string, customColumns []string) error {
	// Get current working directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Make sure it's absolute
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Determine kan root
	paths := config.NewPaths(projectRoot, customLocation)

	// Check if already initialized
	boardsRoot := paths.BoardsRoot()
	if _, err := os.Stat(boardsRoot); err == nil {
		// Already initialized - just register in global config
		return s.registerProject(projectRoot, customLocation)
	}

	// Create directory structure
	if err := os.MkdirAll(boardsRoot, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Use default board name if not specified
	if boardName == "" {
		boardName = defaultBoardName
	}

	// Build columns: use custom if provided, otherwise defaults
	var columns []model.Column
	if len(customColumns) > 0 {
		for i, name := range customColumns {
			columns = append(columns, model.Column{
				Name:  name,
				Color: model.NextColumnColor(i),
			})
		}
	} else {
		columns = model.DefaultColumns()
	}

	// Default column is first in list
	defaultColumn := columns[0].Name

	// Create board
	boardStore := store.NewBoardStore(paths)
	board := &model.BoardConfig{
		ID:            id.Generate(id.Board),
		Name:          boardName,
		Columns:       columns,
		DefaultColumn: defaultColumn,
		CustomFields:  model.DefaultCustomFields(),
		CardDisplay:   model.DefaultCardDisplay(),
	}

	if err := boardStore.Create(board); err != nil {
		return fmt.Errorf("failed to create board: %w", err)
	}

	// Register in global config
	return s.registerProject(projectRoot, customLocation)
}

func (s *InitService) registerProject(projectRoot, customLocation string) error {
	globalCfg, err := s.globalStore.Load()
	if err != nil {
		return err
	}

	// Clean up any stale entries for this path before registering.
	// This handles the case where the user deleted .kan/ and is re-initializing.
	globalCfg.RemoveRepoConfig(projectRoot)

	// Register project
	projectName := filepath.Base(projectRoot)
	globalCfg.RegisterProject(projectName, projectRoot)

	// Always set repo config entry (enables discovery via global config)
	repoCfg := model.RepoConfig{}
	if customLocation != "" {
		repoCfg.DataLocation = customLocation
	}
	globalCfg.SetRepoConfig(projectRoot, repoCfg)

	return s.globalStore.Save(globalCfg)
}
