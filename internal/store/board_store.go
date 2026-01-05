package store

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/version"
)

// FileBoardStore implements BoardStore using the filesystem.
type FileBoardStore struct {
	paths *config.Paths
}

// NewBoardStore creates a new board store.
func NewBoardStore(paths *config.Paths) *FileBoardStore {
	return &FileBoardStore{paths: paths}
}

// Create creates a new board with the given config.
func (s *FileBoardStore) Create(cfg *model.BoardConfig) error {
	if s.Exists(cfg.Name) {
		return kanerr.BoardAlreadyExists(cfg.Name)
	}

	// Validate card_display config references valid custom fields
	if warnings := cfg.ValidateCardDisplay(); len(warnings) > 0 {
		return fmt.Errorf("invalid card_display config: %s", warnings[0])
	}

	cardsDir := s.paths.CardsDir(cfg.Name)

	// Create directories
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		return fmt.Errorf("failed to create board directory: %w", err)
	}

	// Write config
	if err := s.writeConfig(cfg); err != nil {
		return fmt.Errorf("failed to write board config: %w", err)
	}
	return nil
}

// Get reads the board config from disk.
func (s *FileBoardStore) Get(boardName string) (*model.BoardConfig, error) {
	path := s.paths.BoardConfigPath(boardName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, kanerr.BoardNotFound(boardName)
		}
		return nil, fmt.Errorf("failed to read board config: %w", err)
	}

	var cfg model.BoardConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid board config: %w", err)
	}

	// Strict version validation
	if cfg.KanSchema == "" {
		return nil, version.MissingBoardSchema(path)
	}
	if cfg.KanSchema != version.CurrentBoardSchema() {
		return nil, version.InvalidBoardSchema(path, cfg.KanSchema)
	}

	return &cfg, nil
}

// Update writes the board config to disk.
// Note: We don't validate card_display on Update because:
// 1. The config may have been valid before stricter validation was added
// 2. Update is often just adding/removing card IDs, not changing card_display
// Validation happens on Create; invalid configs produce warnings at load time.
func (s *FileBoardStore) Update(cfg *model.BoardConfig) error {
	if err := s.writeConfig(cfg); err != nil {
		return fmt.Errorf("failed to update board config: %w", err)
	}
	return nil
}

// List returns the names of all boards.
func (s *FileBoardStore) List() ([]string, error) {
	boardsRoot := s.paths.BoardsRoot()

	entries, err := os.ReadDir(boardsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // Return empty slice, not nil
		}
		return nil, fmt.Errorf("failed to read boards directory: %w", err)
	}

	var boards []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Verify it has a config.toml
			configPath := s.paths.BoardConfigPath(entry.Name())
			if _, err := os.Stat(configPath); err == nil {
				boards = append(boards, entry.Name())
			}
		}
	}

	if boards == nil {
		boards = []string{} // Ensure non-nil
	}
	return boards, nil
}

// Exists returns true if the board exists.
func (s *FileBoardStore) Exists(boardName string) bool {
	path := s.paths.BoardConfigPath(boardName)
	_, err := os.Stat(path)
	return err == nil
}

func (s *FileBoardStore) writeConfig(cfg *model.BoardConfig) error {
	// Stamp current schema version
	cfg.KanSchema = version.CurrentBoardSchema()

	path := s.paths.BoardConfigPath(cfg.Name)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}
