package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultKanDir     = ".kan"
	BoardsDir         = "boards"
	CardsDir          = "cards"
	ConfigFileName    = "config.toml"
	GlobalConfigDir   = ".config/kan"
	CustomFaviconFile = "favicon.svg"
)

// Paths provides path resolution for Kan data files.
type Paths struct {
	projectRoot  string
	dataLocation string // Custom location from config, empty for default
}

// NewPaths creates a new Paths resolver for the given project.
func NewPaths(projectRoot string, dataLocation string) *Paths {
	return &Paths{
		projectRoot:  projectRoot,
		dataLocation: dataLocation,
	}
}

// KanRoot returns the root directory for Kan data.
func (p *Paths) KanRoot() string {
	if p.dataLocation != "" {
		return filepath.Join(p.projectRoot, p.dataLocation)
	}
	return filepath.Join(p.projectRoot, DefaultKanDir)
}

// BoardsRoot returns the boards directory.
func (p *Paths) BoardsRoot() string {
	return filepath.Join(p.KanRoot(), BoardsDir)
}

// BoardDir returns the directory for a specific board.
func (p *Paths) BoardDir(boardName string) string {
	return filepath.Join(p.BoardsRoot(), boardName)
}

// BoardConfigPath returns the config file path for a board.
func (p *Paths) BoardConfigPath(boardName string) string {
	return filepath.Join(p.BoardDir(boardName), ConfigFileName)
}

// CardsDir returns the cards directory for a board.
func (p *Paths) CardsDir(boardName string) string {
	return filepath.Join(p.BoardDir(boardName), CardsDir)
}

// CardPath returns the file path for a specific card.
func (p *Paths) CardPath(boardName, cardID string) string {
	return filepath.Join(p.CardsDir(boardName), cardID+".json")
}

// ProjectConfigPath returns the path to the project config file.
func (p *Paths) ProjectConfigPath() string {
	return filepath.Join(p.KanRoot(), ConfigFileName)
}

// CustomFaviconPath returns the path to a custom favicon file.
func (p *Paths) CustomFaviconPath() string {
	return filepath.Join(p.KanRoot(), CustomFaviconFile)
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, GlobalConfigDir, ConfigFileName)
}

// GlobalConfigDir returns the directory for global config.
func GlobalConfigDirPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, GlobalConfigDir)
}
