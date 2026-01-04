package model

import (
	"encoding/json"
	"strings"
)

// Card represents a kanban card stored as a JSON file.
// Schema changes require a version bump—see internal/version/version.go.
type Card struct {
	Version         int       `json:"_v"`
	ID              string    `json:"id"`
	Alias           string    `json:"alias"`
	AliasExplicit   bool      `json:"alias_explicit"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	Labels          []string  `json:"labels,omitempty"`
	Parent          string    `json:"parent,omitempty"`
	Creator         string    `json:"creator"`
	CreatedAtMillis int64     `json:"created_at_millis"`
	UpdatedAtMillis int64     `json:"updated_at_millis"`
	Comments        []Comment `json:"comments,omitempty"`

	// Column is computed from board config, not persisted to card files.
	// Populated by service layer when cards are loaded.
	Column string `json:"-"`

	// CustomFields holds board-defined custom fields.
	// These are serialized at the top level of the JSON, not nested.
	CustomFields map[string]any `json:"-"`
}

// Comment represents a comment on a card.
type Comment struct {
	ID              string `json:"id"`
	Body            string `json:"body"`
	Author          string `json:"author"`
	CreatedAtMillis int64  `json:"created_at_millis"`
}

// MarshalJSON implements custom JSON marshaling to merge custom fields
// into the top level of the JSON object.
func (c Card) MarshalJSON() ([]byte, error) {
	// Use an alias to avoid infinite recursion
	type CardAlias Card
	base, err := json.Marshal(CardAlias(c))
	if err != nil {
		return nil, err
	}

	if len(c.CustomFields) == 0 {
		return base, nil
	}

	// Merge custom fields into the base object
	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}

	for k, v := range c.CustomFields {
		merged[k] = v
	}

	return json.Marshal(merged)
}

// UnmarshalJSON implements custom JSON unmarshaling to extract custom fields
// from the top level of the JSON object.
func (c *Card) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion
	type CardAlias Card
	var alias CardAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*c = Card(alias)

	// Extract custom fields (any keys not in the known set)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	knownFields := map[string]bool{
		"_v": true, "id": true, "alias": true, "alias_explicit": true,
		"title": true, "description": true,
		"labels": true, "parent": true, "creator": true,
		"created_at_millis": true, "updated_at_millis": true,
		"comments": true,
	}

	c.CustomFields = make(map[string]any)
	for k, v := range raw {
		if !knownFields[k] {
			var val any
			if err := json.Unmarshal(v, &val); err != nil {
				return err
			}
			c.CustomFields[k] = val
		}
	}

	if len(c.CustomFields) == 0 {
		c.CustomFields = nil
	}

	return nil
}

// ValidateCustomFieldName checks if a custom field name is allowed.
// Returns an error if the name uses a reserved prefix.
func ValidateCustomFieldName(name string) error {
	if strings.HasPrefix(name, "_") {
		return &ReservedFieldPrefixError{FieldName: name, Prefix: "_"}
	}
	if strings.HasPrefix(name, "kan_") {
		return &ReservedFieldPrefixError{FieldName: name, Prefix: "kan_"}
	}
	return nil
}

// ValidateCustomFields checks all custom field names for reserved prefixes.
func ValidateCustomFields(fields map[string]any) error {
	for name := range fields {
		if err := ValidateCustomFieldName(name); err != nil {
			return err
		}
	}
	return nil
}

// ReservedFieldPrefixError indicates a custom field uses a reserved prefix.
type ReservedFieldPrefixError struct {
	FieldName string
	Prefix    string
}

func (e *ReservedFieldPrefixError) Error() string {
	// Suggest alternatives: remove the prefix, or use x_ escape hatch
	suggestion := strings.TrimPrefix(e.FieldName, e.Prefix)
	if suggestion == "" || suggestion == e.FieldName {
		suggestion = "x_" + e.FieldName
	}
	return "custom field \"" + e.FieldName + "\" uses reserved prefix \"" + e.Prefix +
		"\" (reserved for Kan internal use). Try \"" + suggestion + "\" instead."
}
