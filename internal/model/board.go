package model

// Custom field type constants.
const (
	FieldTypeString = "string"
	FieldTypeEnum   = "enum"
	FieldTypeTags   = "tags"
	FieldTypeDate   = "date"
)

// MaxTagsPerField is the maximum number of tags allowed per tags field.
// This prevents accidental abuse and keeps the UI manageable.
const MaxTagsPerField = 10

// ValidFieldTypes lists all supported custom field types.
var ValidFieldTypes = []string{FieldTypeString, FieldTypeEnum, FieldTypeTags, FieldTypeDate}

// IsValidFieldType returns true if the given type is a valid custom field type.
func IsValidFieldType(t string) bool {
	for _, valid := range ValidFieldTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// BoardConfig represents the configuration for a kanban board.
// Stored as config.toml in the board directory.
// Schema changes require a version bump—see internal/version/version.go.
type BoardConfig struct {
	KanSchema     string                       `toml:"kan_schema" json:"kan_schema"`
	ID            string                       `toml:"id" json:"id"`
	Name          string                       `toml:"name" json:"name"`
	Columns       []Column                     `toml:"columns" json:"columns"`
	DefaultColumn string                       `toml:"default_column" json:"default_column"`
	CustomFields  map[string]CustomFieldSchema `toml:"custom_fields,omitempty" json:"custom_fields,omitempty"`
	CardDisplay   CardDisplayConfig            `toml:"card_display,omitempty" json:"card_display,omitempty"`
}

// Column represents a kanban column.
type Column struct {
	Name    string   `toml:"name" json:"name"`
	Color   string   `toml:"color" json:"color"`
	CardIDs []string `toml:"card_ids,omitempty" json:"card_ids,omitempty"`
}

// CustomFieldOption represents a single option for enum/tags fields.
type CustomFieldOption struct {
	Value string `toml:"value" json:"value"`
	Color string `toml:"color,omitempty" json:"color,omitempty"`
}

// CustomFieldSchema defines the schema for a custom field.
type CustomFieldSchema struct {
	Type    string              `toml:"type" json:"type"`                           // "string", "enum", "tags", "date"
	Options []CustomFieldOption `toml:"options,omitempty" json:"options,omitempty"` // For enum/tags types
}

// CardDisplayConfig controls how custom fields render on cards in the board view.
type CardDisplayConfig struct {
	TypeIndicator string   `toml:"type_indicator,omitempty" json:"type_indicator,omitempty"` // enum field shown as badge
	Badges        []string `toml:"badges,omitempty" json:"badges,omitempty"`                 // tags fields shown as chips
	Metadata      []string `toml:"metadata,omitempty" json:"metadata,omitempty"`             // fields shown as small text
}

// DefaultColumns returns the default columns for a new board.
func DefaultColumns() []Column {
	return []Column{
		{Name: "backlog", Color: "#6b7280"},
		{Name: "next", Color: "#3b82f6"},
		{Name: "in-progress", Color: "#f59e0b"},
		{Name: "done", Color: "#10b981"},
	}
}

// DefaultCustomFields returns the default custom fields for a new board.
func DefaultCustomFields() map[string]CustomFieldSchema {
	return map[string]CustomFieldSchema{
		"type": {
			Type: FieldTypeEnum,
			Options: []CustomFieldOption{
				{Value: "bug", Color: "#ef4444"},
				{Value: "enhancement", Color: "#3b82f6"},
				{Value: "feature", Color: "#22c55e"},
				{Value: "chore", Color: "#6b7280"},
			},
		},
	}
}

// DefaultCardDisplay returns the default card display config for a new board.
func DefaultCardDisplay() CardDisplayConfig {
	return CardDisplayConfig{
		TypeIndicator: "type",
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

// ValidateCardDisplay validates that CardDisplayConfig references valid custom fields.
// Returns a list of warning messages for invalid references (non-fatal).
func (b *BoardConfig) ValidateCardDisplay() []string {
	var warnings []string

	cd := b.CardDisplay
	if cd.TypeIndicator == "" && len(cd.Badges) == 0 && len(cd.Metadata) == 0 {
		return nil // Empty config, nothing to validate
	}

	// Validate type_indicator references an enum field
	if cd.TypeIndicator != "" {
		schema, exists := b.CustomFields[cd.TypeIndicator]
		if !exists {
			warnings = append(warnings, "card_display.type_indicator references non-existent field: "+cd.TypeIndicator)
		} else if schema.Type != FieldTypeEnum {
			warnings = append(warnings, "card_display.type_indicator should reference an enum field, but '"+cd.TypeIndicator+"' is type '"+schema.Type+"'")
		}
	}

	// Validate badges reference tags fields
	for _, fieldName := range cd.Badges {
		schema, exists := b.CustomFields[fieldName]
		if !exists {
			warnings = append(warnings, "card_display.badges references non-existent field: "+fieldName)
		} else if schema.Type != FieldTypeTags {
			warnings = append(warnings, "card_display.badges should reference tags fields, but '"+fieldName+"' is type '"+schema.Type+"'")
		}
	}

	// Validate metadata references existing fields
	for _, fieldName := range cd.Metadata {
		if _, exists := b.CustomFields[fieldName]; !exists {
			warnings = append(warnings, "card_display.metadata references non-existent field: "+fieldName)
		}
	}

	return warnings
}
