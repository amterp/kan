package store

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/id"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/version"
)

// FileProjectStore implements ProjectStore using the filesystem.
type FileProjectStore struct {
	paths *config.Paths
}

// NewProjectStore creates a new project store.
func NewProjectStore(paths *config.Paths) *FileProjectStore {
	return &FileProjectStore{paths: paths}
}

// Load reads the project config from disk.
// Returns a default config if the file doesn't exist.
func (s *FileProjectStore) Load() (*model.ProjectConfig, error) {
	path := s.paths.ProjectConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return a minimal default config when file doesn't exist
			return &model.ProjectConfig{
				Name:    "",
				Favicon: model.FaviconConfig{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	var cfg model.ProjectConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid project config: %w", err)
	}

	// Strict version validation (only if file exists and has content)
	if cfg.KanSchema == "" {
		return nil, version.MissingProjectSchema(path)
	}
	if cfg.KanSchema != version.CurrentProjectSchema() {
		return nil, version.InvalidProjectSchema(path, cfg.KanSchema)
	}

	return &cfg, nil
}

// Save writes the project config to disk.
func (s *FileProjectStore) Save(cfg *model.ProjectConfig) error {
	// Stamp current schema version
	cfg.KanSchema = version.CurrentProjectSchema()

	path := s.paths.ProjectConfigPath()

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create project config: %w", err)
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}

// Exists returns true if the project config file exists.
func (s *FileProjectStore) Exists() bool {
	path := s.paths.ProjectConfigPath()
	_, err := os.Stat(path)
	return err == nil
}

// EnsureInitialized ensures the project config exists and has an ID.
// If the config file doesn't exist, creates one with the given name.
// If the config exists but has no ID, generates one and saves.
// This provides a graceful upgrade path for projects created before project configs existed.
func (s *FileProjectStore) EnsureInitialized(defaultName string) error {
	path := s.paths.ProjectConfigPath()

	// Try to read existing config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file - create one with defaults
			cfg := &model.ProjectConfig{
				ID:   id.Generate(id.Project),
				Name: defaultName,
			}
			cfg.Favicon = model.DefaultFaviconConfig(cfg.ID, cfg.Name)
			return s.Save(cfg)
		}
		return fmt.Errorf("failed to read project config: %w", err)
	}

	// Parse existing config
	var cfg model.ProjectConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid project config: %w", err)
	}

	// Check if we need to generate an ID
	if cfg.ID == "" {
		cfg.ID = id.Generate(id.Project)

		// If name is also empty, set the default
		if cfg.Name == "" {
			cfg.Name = defaultName
		}

		// Only regenerate favicon if it's empty/unconfigured.
		// This preserves any user customizations made before IDs existed.
		if cfg.Favicon.Background == "" {
			cfg.Favicon = model.DefaultFaviconConfig(cfg.ID, cfg.Name)
		}

		return s.Save(&cfg)
	}

	return nil
}
