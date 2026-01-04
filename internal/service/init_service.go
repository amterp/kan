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
func (s *InitService) Initialize(customLocation string) error {
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

	// Create default board
	boardStore := store.NewBoardStore(paths)
	defaultBoard := &model.BoardConfig{
		ID:            id.Generate(),
		Name:          defaultBoardName,
		Columns:       model.DefaultColumns(),
		DefaultColumn: "Backlog",
	}

	if err := boardStore.Create(defaultBoard); err != nil {
		return fmt.Errorf("failed to create default board: %w", err)
	}

	// Register in global config
	return s.registerProject(projectRoot, customLocation)
}

func (s *InitService) registerProject(projectRoot, customLocation string) error {
	globalCfg, err := s.globalStore.Load()
	if err != nil {
		return err
	}

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
