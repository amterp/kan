package store

import "github.com/amterp/kan/internal/model"

// CardStore handles card persistence.
type CardStore interface {
	Create(boardName string, card *model.Card) error
	Get(boardName, cardID string) (*model.Card, error)
	Update(boardName string, card *model.Card) error
	Delete(boardName, cardID string) error
	List(boardName string) ([]*model.Card, error)
	FindByAlias(boardName, alias string) (*model.Card, error)
}

// BoardStore handles board persistence.
type BoardStore interface {
	Create(config *model.BoardConfig) error
	Get(boardName string) (*model.BoardConfig, error)
	Update(config *model.BoardConfig) error
	Delete(boardName string) error
	List() ([]string, error) // Returns board names
	Exists(boardName string) bool
}

// GlobalStore handles global config persistence.
type GlobalStore interface {
	Load() (*model.GlobalConfig, error)
	Save(config *model.GlobalConfig) error
	EnsureExists() error
}

// ProjectStore handles project-level config persistence.
type ProjectStore interface {
	Load() (*model.ProjectConfig, error)
	Save(config *model.ProjectConfig) error
	Exists() bool
	EnsureInitialized(defaultName string) error
}
