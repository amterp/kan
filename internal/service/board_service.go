package service

import (
	"regexp"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/id"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// columnNameRegex validates column names: lowercase alphanumeric and hyphens.
var columnNameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// BoardService handles board operations.
type BoardService struct {
	boardStore store.BoardStore
	cardStore  store.CardStore
}

// NewBoardService creates a new board service.
func NewBoardService(boardStore store.BoardStore, cardStore store.CardStore) *BoardService {
	return &BoardService{
		boardStore: boardStore,
		cardStore:  cardStore,
	}
}

// Create creates a new board with default columns.
func (s *BoardService) Create(name string) error {
	cfg := &model.BoardConfig{
		ID:            id.Generate(id.Board),
		Name:          name,
		Columns:       model.DefaultColumns(),
		DefaultColumn: "backlog",
		CustomFields:  model.DefaultCustomFields(),
		CardDisplay:   model.DefaultCardDisplay(),
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

// AddColumn adds a new column to a board.
// If color is empty, auto-assigns from the color palette.
// If position is -1, appends to end.
func (s *BoardService) AddColumn(boardName, columnName, color string, position int) error {
	// Validate column name format
	if !columnNameRegex.MatchString(columnName) {
		return kanerr.InvalidField("column name", "must be lowercase alphanumeric with hyphens (e.g., 'in-progress')")
	}

	cfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return err
	}

	// Auto-assign color if not specified
	if color == "" {
		color = model.NextColumnColor(len(cfg.Columns))
	}

	if !cfg.AddColumn(columnName, color, position) {
		return kanerr.ColumnAlreadyExists(columnName, boardName)
	}

	return s.boardStore.Update(cfg)
}

// DeleteColumn removes a column and all its cards.
// Returns the number of cards deleted.
func (s *BoardService) DeleteColumn(boardName, columnName string) (int, error) {
	cfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return 0, err
	}

	// Cannot delete the last column
	if len(cfg.Columns) <= 1 {
		return 0, kanerr.InvalidField("column", "cannot delete the last remaining column")
	}

	// Cannot delete the default column
	if cfg.DefaultColumn == columnName {
		return 0, kanerr.InvalidField("column", "cannot delete the default column; change default_column first")
	}

	// Check column exists
	if !cfg.HasColumn(columnName) {
		return 0, kanerr.ColumnNotFound(columnName, boardName)
	}

	// Get card IDs before removing
	col := cfg.GetColumn(columnName)
	cardIDs := col.CardIDs

	// Delete all cards in the column
	for _, cardID := range cardIDs {
		// Ignore errors on individual card deletions
		_ = s.cardStore.Delete(boardName, cardID)
	}

	// Remove column from config
	cfg.RemoveColumn(columnName)

	if err := s.boardStore.Update(cfg); err != nil {
		return len(cardIDs), err
	}

	return len(cardIDs), nil
}

// RenameColumn renames a column.
// Also updates default_column if it referenced the old name.
func (s *BoardService) RenameColumn(boardName, oldName, newName string) error {
	// Validate new column name format
	if !columnNameRegex.MatchString(newName) {
		return kanerr.InvalidField("column name", "must be lowercase alphanumeric with hyphens (e.g., 'in-progress')")
	}

	cfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return err
	}

	if !cfg.HasColumn(oldName) {
		return kanerr.ColumnNotFound(oldName, boardName)
	}

	if !cfg.RenameColumn(oldName, newName) {
		return kanerr.ColumnAlreadyExists(newName, boardName)
	}

	return s.boardStore.Update(cfg)
}

// UpdateColumnColor updates a column's color.
func (s *BoardService) UpdateColumnColor(boardName, columnName, color string) error {
	cfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return err
	}

	if !cfg.SetColumnColor(columnName, color) {
		return kanerr.ColumnNotFound(columnName, boardName)
	}

	return s.boardStore.Update(cfg)
}

// ReorderColumn moves a column to a new position (0-indexed).
func (s *BoardService) ReorderColumn(boardName, columnName string, newPosition int) error {
	cfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return err
	}

	if !cfg.HasColumn(columnName) {
		return kanerr.ColumnNotFound(columnName, boardName)
	}

	if newPosition < 0 || newPosition >= len(cfg.Columns) {
		return kanerr.InvalidField("position", "must be between 0 and number of columns minus 1")
	}

	cfg.MoveColumn(columnName, newPosition)

	return s.boardStore.Update(cfg)
}

// ReorderColumns reorders all columns according to the provided order.
// The columnNames slice must contain exactly the same column names as exist in the board.
func (s *BoardService) ReorderColumns(boardName string, columnNames []string) error {
	cfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return err
	}

	// Validate that all columns are present
	if len(columnNames) != len(cfg.Columns) {
		return kanerr.InvalidField("columns", "must contain exactly all existing column names")
	}

	// Build a map of column name -> column data
	colMap := make(map[string]model.Column)
	for _, col := range cfg.Columns {
		colMap[col.Name] = col
	}

	// Build new column order
	newColumns := make([]model.Column, 0, len(columnNames))
	for _, name := range columnNames {
		col, exists := colMap[name]
		if !exists {
			return kanerr.ColumnNotFound(name, boardName)
		}
		newColumns = append(newColumns, col)
		delete(colMap, name)
	}

	// Check for duplicates (colMap should be empty now)
	if len(colMap) > 0 {
		return kanerr.InvalidField("columns", "contains duplicate or missing column names")
	}

	cfg.Columns = newColumns
	return s.boardStore.Update(cfg)
}

// GetColumnCardCount returns the number of cards in a column.
func (s *BoardService) GetColumnCardCount(boardName, columnName string) (int, error) {
	cfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return 0, err
	}

	col := cfg.GetColumn(columnName)
	if col == nil {
		return 0, kanerr.ColumnNotFound(columnName, boardName)
	}

	return len(col.CardIDs), nil
}
