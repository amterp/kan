package model

import (
	"encoding/json"
)

// Card represents a kanban card stored as a JSON file.
type Card struct {
	ID              string    `json:"id"`
	Alias           string    `json:"alias"`
	AliasExplicit   bool      `json:"alias_explicit"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	Column          string    `json:"column"`
	Labels          []string  `json:"labels,omitempty"`
	Parent          string    `json:"parent,omitempty"`
	Creator         string    `json:"creator"`
	CreatedAtMillis int64     `json:"created_at_millis"`
	UpdatedAtMillis int64     `json:"updated_at_millis"`
	Comments        []Comment `json:"comments,omitempty"`

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
		"id": true, "alias": true, "alias_explicit": true,
		"title": true, "description": true, "column": true,
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
