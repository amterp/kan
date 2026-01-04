package model

// GlobalConfig represents the user's global Kan configuration.
// Stored at ~/.config/kan/config.toml
// Schema changes require a version bump—see internal/version/version.go.
type GlobalConfig struct {
	KanSchema string                `toml:"kan_schema"`
	Editor    string                `toml:"editor,omitempty"`
	Projects  map[string]string     `toml:"projects,omitempty"` // name -> path
	Repos     map[string]RepoConfig `toml:"repos,omitempty"`    // path -> config
}

// RepoConfig holds per-repository settings.
type RepoConfig struct {
	DefaultBoard string `toml:"default_board,omitempty"`
	DataLocation string `toml:"data_location,omitempty"` // Custom .kan location
}

// GetRepoConfig returns the config for a given repo path, or nil if none.
func (g *GlobalConfig) GetRepoConfig(repoPath string) *RepoConfig {
	if g.Repos == nil {
		return nil
	}
	if cfg, ok := g.Repos[repoPath]; ok {
		return &cfg
	}
	return nil
}

// SetRepoConfig sets the config for a given repo path.
func (g *GlobalConfig) SetRepoConfig(repoPath string, cfg RepoConfig) {
	if g.Repos == nil {
		g.Repos = make(map[string]RepoConfig)
	}
	g.Repos[repoPath] = cfg
}

// RegisterProject adds a project to the registry.
func (g *GlobalConfig) RegisterProject(name, path string) {
	if g.Projects == nil {
		g.Projects = make(map[string]string)
	}
	g.Projects[name] = path
}
