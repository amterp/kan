package model

// BoardConfig represents the configuration for a kanban board.
// Stored as config.toml in the board directory.
type BoardConfig struct {
	ID            string              `toml:"id"`
	Name          string              `toml:"name"`
	Columns       []Column            `toml:"columns"`
	DefaultColumn string              `toml:"default_column"`
	Labels        []Label             `toml:"labels,omitempty"`
	CustomFields  map[string]CustomFieldSchema `toml:"custom_fields,omitempty"`
}

// Column represents a kanban column.
type Column struct {
	Name  string `toml:"name"`
	Color string `toml:"color"`
}

// Label represents a card label.
type Label struct {
	Name        string `toml:"name"`
	Color       string `toml:"color"`
	Description string `toml:"description,omitempty"`
}

// CustomFieldSchema defines the schema for a custom field.
type CustomFieldSchema struct {
	Type   string   `toml:"type"`             // "string", "enum", "date"
	Values []string `toml:"values,omitempty"` // For enum type
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
