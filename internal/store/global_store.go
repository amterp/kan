package store

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/version"
)

// FileGlobalStore implements GlobalStore using the filesystem.
type FileGlobalStore struct{}

// NewGlobalStore creates a new global store.
func NewGlobalStore() *FileGlobalStore {
	return &FileGlobalStore{}
}

// Load reads the global config from disk.
// Returns an empty config if the file doesn't exist.
func (s *FileGlobalStore) Load() (*model.GlobalConfig, error) {
	path := config.GlobalConfigPath()
	if path == "" {
		return &model.GlobalConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.GlobalConfig{}, nil
		}
		return nil, err
	}

	var cfg model.GlobalConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Strict version validation (only if file exists)
	if cfg.KanSchema == "" {
		return nil, version.MissingGlobalSchema(path)
	}
	if cfg.KanSchema != version.CurrentGlobalSchema() {
		return nil, version.InvalidGlobalSchema(path, cfg.KanSchema)
	}

	return &cfg, nil
}

// Save writes the global config to disk.
func (s *FileGlobalStore) Save(cfg *model.GlobalConfig) error {
	// Stamp current schema version
	cfg.KanSchema = version.CurrentGlobalSchema()

	path := config.GlobalConfigPath()
	if path == "" {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}

// EnsureExists creates the global config file if it doesn't exist.
func (s *FileGlobalStore) EnsureExists() error {
	path := config.GlobalConfigPath()
	if path == "" {
		return nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return s.Save(&model.GlobalConfig{})
	}
	return nil
}
