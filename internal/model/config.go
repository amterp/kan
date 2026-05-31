package model

// GlobalConfig represents the user's global Kan configuration.
// Stored at ~/.config/kan/config.toml
// Schema changes require a version bump—see internal/version/version.go.
type GlobalConfig struct {
	KanSchema   string                `toml:"kan_schema"`
	Editor      string                `toml:"editor,omitempty"`
	Projects    map[string]string     `toml:"projects,omitempty"`     // name -> path
	Repos       map[string]RepoConfig `toml:"repos,omitempty"`        // path -> config
	GlobalBoard *GlobalBoardRef       `toml:"global_board,omitempty"` // board reachable from anywhere via -g
}

// RepoConfig holds per-repository settings.
type RepoConfig struct {
	DefaultBoard string `toml:"default_board,omitempty"`
	DataLocation string `toml:"data_location,omitempty"` // Custom .kan location
}

// GlobalBoardRef designates a single board that `kan ... -g` targets regardless
// of the working directory. Path is the project root (a key into Repos, which
// supplies any custom DataLocation); Board is the board name within it.
type GlobalBoardRef struct {
	Path  string `toml:"path"`
	Board string `toml:"board"`
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

// SetGlobalBoard designates the board that `-g` targets.
func (g *GlobalConfig) SetGlobalBoard(path, board string) {
	g.GlobalBoard = &GlobalBoardRef{Path: path, Board: board}
}

// ClearGlobalBoard removes the global board designation.
func (g *GlobalConfig) ClearGlobalBoard() {
	g.GlobalBoard = nil
}

// RemoveRepoConfig removes a repo config and any project entries pointing to that path.
// Used to clean up stale entries when re-initializing a project.
func (g *GlobalConfig) RemoveRepoConfig(path string) {
	delete(g.Repos, path)
	// Also remove any Projects entries pointing to this path
	for name, p := range g.Projects {
		if p == path {
			delete(g.Projects, name)
		}
	}
}
