package service

import (
	"fmt"
	"os"
	"path/filepath"

	fid "github.com/amterp/flexid"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/git"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

const defaultBoardName = "main"

// InitService handles repository initialization.
type InitService struct {
	gitClient   *git.Client
	globalStore store.GlobalStore
}

// NewInitService creates a new init service.
func NewInitService(gitClient *git.Client, globalStore store.GlobalStore) *InitService {
	return &InitService{
		gitClient:   gitClient,
		globalStore: globalStore,
	}
}

// Initialize initializes Kan in the current repository.
// If customLocation is empty, uses the default .kan directory.
func (s *InitService) Initialize(customLocation string) error {
	// Verify we're in a git repository
	repoRoot, err := s.gitClient.GetRepoRoot()
	if err != nil {
		return fmt.Errorf("must be in a git repository: %w", err)
	}

	// Determine kan root
	paths := config.NewPaths(repoRoot, customLocation)

	// Check if already initialized
	boardsRoot := paths.BoardsRoot()
	if _, err := os.Stat(boardsRoot); err == nil {
		// Already initialized - just register in global config
		return s.registerRepo(repoRoot, customLocation)
	}

	// Create directory structure
	if err := os.MkdirAll(boardsRoot, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create default board
	boardStore := store.NewBoardStore(paths)
	defaultBoard := &model.BoardConfig{
		ID:            fid.MustGenerate(),
		Name:          defaultBoardName,
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
	}

	if err := boardStore.Create(defaultBoard); err != nil {
		return fmt.Errorf("failed to create default board: %w", err)
	}

	// Register in global config
	return s.registerRepo(repoRoot, customLocation)
}

func (s *InitService) registerRepo(repoRoot, customLocation string) error {
	globalCfg, err := s.globalStore.Load()
	if err != nil {
		return err
	}

	// Register project
	projectName := filepath.Base(repoRoot)
	globalCfg.RegisterProject(projectName, repoRoot)

	// Set repo config if custom location
	if customLocation != "" {
		globalCfg.SetRepoConfig(repoRoot, model.RepoConfig{
			DataLocation: customLocation,
		})
	}

	return s.globalStore.Save(globalCfg)
}
