package service

import (
	fid "github.com/amterp/flexid"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// BoardService handles board operations.
type BoardService struct {
	boardStore store.BoardStore
}

// NewBoardService creates a new board service.
func NewBoardService(boardStore store.BoardStore) *BoardService {
	return &BoardService{boardStore: boardStore}
}

// Create creates a new board with default columns.
func (s *BoardService) Create(name string) error {
	cfg := &model.BoardConfig{
		ID:            fid.MustGenerate(),
		Name:          name,
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
	}

	// Store.Create handles existence check and returns proper error
	return s.boardStore.Create(cfg)
}

// List returns the names of all boards.
func (s *BoardService) List() ([]string, error) {
	return s.boardStore.List()
}

// Get returns the board configuration.
func (s *BoardService) Get(name string) (*model.BoardConfig, error) {
	return s.boardStore.Get(name)
}

// Exists returns true if the board exists.
func (s *BoardService) Exists(name string) bool {
	return s.boardStore.Exists(name)
}
