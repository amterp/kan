package model

import (
	"fmt"
	"regexp"
)

// Custom field type constants.
const (
	FieldTypeString  = "string"
	FieldTypeEnum    = "enum"
	FieldTypeEnumSet = "enum-set"
	FieldTypeFreeSet = "free-set"
	FieldTypeDate    = "date"
)

// MaxSetItems is the maximum number of values allowed per set field (enum-set, free-set).
// This prevents accidental abuse and keeps the UI manageable.
const MaxSetItems = 10

// ValidFieldTypes lists all supported custom field types.
var ValidFieldTypes = []string{FieldTypeString, FieldTypeEnum, FieldTypeEnumSet, FieldTypeFreeSet, FieldTypeDate}

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
	LinkRules     []LinkRule                   `toml:"link_rules,omitempty" json:"link_rules,omitempty"`
	PatternHooks  []PatternHook                `toml:"pattern_hooks,omitempty" json:"pattern_hooks,omitempty"`
}

// Column represents a kanban column.
type Column struct {
	Name        string   `toml:"name" json:"name"`
	Color       string   `toml:"color" json:"color"`
	Description string   `toml:"description,omitempty" json:"description,omitempty"`
	Limit       int      `toml:"limit,omitempty" json:"limit,omitempty"`
	CardIDs     []string `toml:"card_ids,omitempty" json:"card_ids,omitempty"`
}

// IsAtLimit returns true if the column has a limit and has reached it.
func (c *Column) IsAtLimit() bool {
	return c.Limit > 0 && len(c.CardIDs) >= c.Limit
}

// CustomFieldOption represents a single option for enum/enum-set fields.
type CustomFieldOption struct {
	Value       string `toml:"value" json:"value"`
	Color       string `toml:"color,omitempty" json:"color,omitempty"`
	Description string `toml:"description,omitempty" json:"description,omitempty"`
}

// CustomFieldSchema defines the schema for a custom field.
type CustomFieldSchema struct {
	Type        string              `toml:"type" json:"type"`                           // "string", "enum", "enum-set", "free-set", "date"
	Options     []CustomFieldOption `toml:"options,omitempty" json:"options,omitempty"` // For enum/enum-set types
	Wanted      bool                `toml:"wanted,omitempty" json:"wanted,omitempty"`   // Warn if field is missing
	Description string              `toml:"description,omitempty" json:"description,omitempty"`
}

// CardDisplayConfig controls how custom fields render on cards in the board view.
type CardDisplayConfig struct {
	TypeIndicator string   `toml:"type_indicator,omitempty" json:"type_indicator,omitempty"` // enum field shown as badge
	Badges        []string `toml:"badges,omitempty" json:"badges,omitempty"`                 // set fields shown as chips
	Metadata      []string `toml:"metadata,omitempty" json:"metadata,omitempty"`             // fields shown as small text
}

// LinkRule defines a pattern-based auto-link rule.
// Text matching the pattern will be converted to clickable links.
type LinkRule struct {
	Name    string `toml:"name" json:"name"`       // Human-readable name for the rule (e.g., "Jira")
	Pattern string `toml:"pattern" json:"pattern"` // Regex pattern with capture groups
	URL     string `toml:"url" json:"url"`         // URL template using {0} for full match, {1}, {2}, etc. for groups
}

// PatternHook defines a hook that runs when cards are created with matching titles.
// The command receives the card ID and board name as arguments.
type PatternHook struct {
	Name         string `toml:"name" json:"name"`                           // Human-readable name for the hook
	PatternTitle string `toml:"pattern_title" json:"pattern_title"`         // Regex pattern to match card titles
	Command      string `toml:"command" json:"command"`                     // Command to execute (~ expanded)
	Timeout      int    `toml:"timeout,omitempty" json:"timeout,omitempty"` // Timeout in seconds (default: 30)
}

// ValidateLinkRules validates that all link rules have valid regex patterns.
// Returns a list of warning messages for invalid patterns (non-fatal).
func ValidateLinkRules(rules []LinkRule) []string {
	var warnings []string
	for _, rule := range rules {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"link_rules: invalid regex in '%s': %s", rule.Name, err.Error()))
		}
	}
	return warnings
}

// ValidatePatternHooks validates that all pattern hooks have valid regex patterns.
// Returns a list of warning messages for invalid patterns (non-fatal).
func ValidatePatternHooks(hooks []PatternHook) []string {
	var warnings []string
	for _, hook := range hooks {
		if hook.Name == "" {
			warnings = append(warnings, "pattern_hooks: hook missing required 'name' field")
			continue
		}
		if hook.PatternTitle == "" {
			warnings = append(warnings, fmt.Sprintf(
				"pattern_hooks: hook '%s' missing required 'pattern_title' field", hook.Name))
		} else if _, err := regexp.Compile(hook.PatternTitle); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"pattern_hooks: invalid regex in '%s': %s", hook.Name, err.Error()))
		}
		if hook.Command == "" {
			warnings = append(warnings, fmt.Sprintf(
				"pattern_hooks: hook '%s' missing required 'command' field", hook.Name))
		}
	}
	return warnings
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
				{Value: "bug", Color: "#dc2626"},
				{Value: "enhancement", Color: "#2563eb"},
				{Value: "feature", Color: "#16a34a"},
				{Value: "chore", Color: "#4b5563"},
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

// GetColumn returns a pointer to the column with the given name, or nil if not found.
func (b *BoardConfig) GetColumn(name string) *Column {
	for i := range b.Columns {
		if b.Columns[i].Name == name {
			return &b.Columns[i]
		}
	}
	return nil
}

// AddColumn adds a new column at the specified position.
// If position is -1 or >= len(columns), appends to end.
// Returns false if a column with the same name already exists.
func (b *BoardConfig) AddColumn(name, color string, position int) bool {
	if b.HasColumn(name) {
		return false
	}

	newCol := Column{Name: name, Color: color}

	// Append to end if position is -1 or out of bounds
	if position < 0 || position >= len(b.Columns) {
		b.Columns = append(b.Columns, newCol)
		return true
	}

	// Insert at position
	b.Columns = append(b.Columns[:position], append([]Column{newCol}, b.Columns[position:]...)...)
	return true
}

// RemoveColumn removes a column by name.
// Returns the removed column's card IDs (for cleanup), or nil if column not found.
// Does not delete the card files - caller is responsible for that.
func (b *BoardConfig) RemoveColumn(name string) []string {
	idx := b.GetColumnIndex(name)
	if idx < 0 {
		return nil
	}

	cardIDs := b.Columns[idx].CardIDs
	b.Columns = append(b.Columns[:idx], b.Columns[idx+1:]...)
	return cardIDs
}

// RenameColumn renames a column.
// Returns false if the old column doesn't exist or new name already exists.
func (b *BoardConfig) RenameColumn(oldName, newName string) bool {
	if oldName == newName {
		return true // No-op
	}
	if b.HasColumn(newName) {
		return false // New name already exists
	}

	idx := b.GetColumnIndex(oldName)
	if idx < 0 {
		return false // Old column doesn't exist
	}

	b.Columns[idx].Name = newName

	// Update default_column if it referenced the old name
	if b.DefaultColumn == oldName {
		b.DefaultColumn = newName
	}

	return true
}

// SetColumnColor updates a column's color.
// Returns false if the column doesn't exist.
func (b *BoardConfig) SetColumnColor(name, color string) bool {
	col := b.GetColumn(name)
	if col == nil {
		return false
	}
	col.Color = color
	return true
}

// SetColumnLimit updates a column's card limit.
// A limit of 0 clears the limit. Returns false if the column doesn't exist.
func (b *BoardConfig) SetColumnLimit(name string, limit int) bool {
	col := b.GetColumn(name)
	if col == nil {
		return false
	}
	col.Limit = limit
	return true
}

// SetColumnDescription updates a column's description.
// Returns false if the column doesn't exist.
func (b *BoardConfig) SetColumnDescription(name, description string) bool {
	col := b.GetColumn(name)
	if col == nil {
		return false
	}
	col.Description = description
	return true
}

// MoveColumn moves a column to a new position.
// Returns false if the column doesn't exist or position is invalid.
func (b *BoardConfig) MoveColumn(name string, newPosition int) bool {
	oldIdx := b.GetColumnIndex(name)
	if oldIdx < 0 {
		return false
	}

	// Validate new position
	if newPosition < 0 || newPosition >= len(b.Columns) {
		return false
	}

	if oldIdx == newPosition {
		return true // No-op
	}

	// Remove from old position
	col := b.Columns[oldIdx]
	b.Columns = append(b.Columns[:oldIdx], b.Columns[oldIdx+1:]...)

	// Insert at new position
	b.Columns = append(b.Columns[:newPosition], append([]Column{col}, b.Columns[newPosition:]...)...)
	return true
}

// GetOptionColor returns the color for an enum option value, or empty string if not found.
func (b *BoardConfig) GetOptionColor(fieldName, value string) string {
	schema, exists := b.CustomFields[fieldName]
	if !exists {
		return ""
	}
	for _, opt := range schema.Options {
		if opt.Value == value {
			return opt.Color
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

	// Validate badges reference set fields (enum-set or free-set)
	for _, fieldName := range cd.Badges {
		schema, exists := b.CustomFields[fieldName]
		if !exists {
			warnings = append(warnings, "card_display.badges references non-existent field: "+fieldName)
		} else if schema.Type != FieldTypeEnumSet && schema.Type != FieldTypeFreeSet {
			warnings = append(warnings, "card_display.badges should reference set fields (enum-set or free-set), but '"+fieldName+"' is type '"+schema.Type+"'")
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
