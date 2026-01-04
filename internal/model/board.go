package model

// BoardConfig represents the configuration for a kanban board.
// Stored as config.toml in the board directory.
type BoardConfig struct {
	KanSchema     string                       `toml:"kan_schema" json:"kan_schema"`
	ID            string                       `toml:"id" json:"id"`
	Name          string                       `toml:"name" json:"name"`
	Columns       []Column                     `toml:"columns" json:"columns"`
	DefaultColumn string                       `toml:"default_column" json:"default_column"`
	Labels        []Label                      `toml:"labels,omitempty" json:"labels,omitempty"`
	CustomFields  map[string]CustomFieldSchema `toml:"custom_fields,omitempty" json:"custom_fields,omitempty"`
}

// Column represents a kanban column.
type Column struct {
	Name    string   `toml:"name" json:"name"`
	Color   string   `toml:"color" json:"color"`
	CardIDs []string `toml:"card_ids,omitempty" json:"card_ids,omitempty"`
}

// Label represents a card label.
type Label struct {
	Name        string `toml:"name" json:"name"`
	Color       string `toml:"color" json:"color"`
	Description string `toml:"description,omitempty" json:"description,omitempty"`
}

// CustomFieldSchema defines the schema for a custom field.
type CustomFieldSchema struct {
	Type   string   `toml:"type" json:"type"`                       // "string", "enum", "date"
	Values []string `toml:"values,omitempty" json:"values,omitempty"` // For enum type
}

// DefaultColumns returns the default columns for a new board.
func DefaultColumns() []Column {
	return []Column{
		{Name: "Backlog", Color: "#6b7280"},
		{Name: "Next", Color: "#3b82f6"},
		{Name: "In Progress", Color: "#f59e0b"},
		{Name: "Done", Color: "#10b981"},
	}
}

// HasColumn returns true if the board has a column with the given name.
func (b *BoardConfig) HasColumn(name string) bool {
	for _, col := range b.Columns {
		if col.Name == name {
			return true
		}
	}
	return false
}

// HasLabel returns true if the board has a label with the given name.
func (b *BoardConfig) HasLabel(name string) bool {
	for _, lbl := range b.Labels {
		if lbl.Name == name {
			return true
		}
	}
	return false
}

// GetDefaultColumn returns the default column name.
// Falls back to the first column if default_column is not set.
func (b *BoardConfig) GetDefaultColumn() string {
	if b.DefaultColumn != "" {
		return b.DefaultColumn
	}
	if len(b.Columns) > 0 {
		return b.Columns[0].Name
	}
	return ""
}

// GetColumnIndex returns the index of the column with the given name, or -1 if not found.
func (b *BoardConfig) GetColumnIndex(name string) int {
	for i, col := range b.Columns {
		if col.Name == name {
			return i
		}
	}
	return -1
}

// AddCardToColumn adds a card ID to a column's card list at the end.
// Returns false if the column doesn't exist.
func (b *BoardConfig) AddCardToColumn(cardID, columnName string) bool {
	return b.InsertCardInColumn(cardID, columnName, -1)
}

// InsertCardInColumn adds a card ID to a column's card list at a specific position.
// If position is -1 or >= len(cards), appends to end.
// Returns false if the column doesn't exist.
func (b *BoardConfig) InsertCardInColumn(cardID, columnName string, position int) bool {
	idx := b.GetColumnIndex(columnName)
	if idx < 0 {
		return false
	}

	cards := b.Columns[idx].CardIDs

	// Append to end if position is -1 or out of bounds
	if position < 0 || position >= len(cards) {
		b.Columns[idx].CardIDs = append(cards, cardID)
		return true
	}

	// Insert at position
	newCards := make([]string, 0, len(cards)+1)
	newCards = append(newCards, cards[:position]...)
	newCards = append(newCards, cardID)
	newCards = append(newCards, cards[position:]...)
	b.Columns[idx].CardIDs = newCards
	return true
}

// RemoveCardFromColumn removes a card ID from a column's card list.
// Returns the column name if found, empty string if not found.
func (b *BoardConfig) RemoveCardFromColumn(cardID string) string {
	for i, col := range b.Columns {
		for j, id := range col.CardIDs {
			if id == cardID {
				// Remove the card ID
				b.Columns[i].CardIDs = append(col.CardIDs[:j], col.CardIDs[j+1:]...)
				return col.Name
			}
		}
	}
	return ""
}

// MoveCardToColumn moves a card from its current column to a new column at the end.
// Returns false if the target column doesn't exist.
func (b *BoardConfig) MoveCardToColumn(cardID, targetColumn string) bool {
	return b.MoveCardToColumnAt(cardID, targetColumn, -1)
}

// MoveCardToColumnAt moves a card from its current column to a new column at a specific position.
// If position is -1, appends to end.
// Returns false if the target column doesn't exist.
func (b *BoardConfig) MoveCardToColumnAt(cardID, targetColumn string, position int) bool {
	// First remove from current column (if any)
	b.RemoveCardFromColumn(cardID)
	// Then insert at position in target column
	return b.InsertCardInColumn(cardID, targetColumn, position)
}

// GetCardColumn returns the column name containing the given card ID.
// Returns empty string if the card is not found in any column.
func (b *BoardConfig) GetCardColumn(cardID string) string {
	for _, col := range b.Columns {
		for _, id := range col.CardIDs {
			if id == cardID {
				return col.Name
			}
		}
	}
	return ""
}
