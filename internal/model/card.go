package model

import (
	"bytes"
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
	Parent          string    `json:"parent,omitempty"`
	Creator         string    `json:"creator"`
	CreatedAtMillis int64     `json:"created_at_millis"`
	UpdatedAtMillis int64     `json:"updated_at_millis"`
	Comments        []Comment `json:"comments,omitempty"`

	// History is an append-only, chronological log of tracked field changes.
	// Today only column transitions are recorded; the structure is general so
	// other fields can be tracked later without a schema migration. See
	// HistoryEntry. Git tracks content history (title/description); Kan tracks
	// state-transition timing, which git can't reconstruct from commit cadence.
	History []HistoryEntry `json:"history,omitempty"`

	// Column is the column this card belongs to. This is the single source of
	// truth for card-column membership (board config no longer stores card_ids).
	Column string `json:"column"`

	// Position is a fractional index string for ordering within the column.
	// Cards are sorted by this field lexicographically.
	Position string `json:"position"`

	// CustomFields holds board-defined custom fields (including labels, type, etc.).
	// These are serialized at the top level of the JSON, not nested.
	CustomFields map[string]any `json:"-"`
}

// HistoryEntry records a single tracked field change on a card.
//
// Field is the changed field name ("column" for now); Value is the NEW value
// it became; At is event-time in Unix millis. Entries are append-only and
// chronological (oldest first). The duration a value was held equals the At of
// the next entry with the same Field, minus this entry's At (for the latest
// entry, "now" minus its At).
//
// Value is `any` deliberately: it holds a string today, but future set-type
// fields (enum-set/free-set) can store arrays here without a schema migration.
type HistoryEntry struct {
	Field string `json:"field"`
	Value any    `json:"value"`
	At    int64  `json:"at"`
}

// Comment represents a comment on a card.
type Comment struct {
	ID              string `json:"id"`
	Body            string `json:"body"`
	Author          string `json:"author"`
	CreatedAtMillis int64  `json:"created_at_millis"`
	UpdatedAtMillis int64  `json:"updated_at_millis,omitempty"`
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

	c.CustomFields = make(map[string]any)
	for k, v := range raw {
		if !reservedCardFieldNames[k] {
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

// MarshalFile renders the card as it is stored on disk: pretty-printed with
// 2-space indentation, except each history entry is collapsed onto a single
// line. Because history is append-only and chronological, appending a
// transition is then a clean one-line diff for any VCS, while the rest of the
// card stays human-readable. Use this (not json.MarshalIndent) for writes.
func (c Card) MarshalFile() ([]byte, error) {
	// Render the card without history, pretty-printed. History is spliced back
	// in below so we control its layout precisely (json.MarshalIndent would
	// otherwise spread each entry across several lines).
	withoutHistory := c
	withoutHistory.History = nil
	body, err := json.MarshalIndent(withoutHistory, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(c.History) == 0 {
		return body, nil
	}

	var block strings.Builder
	block.WriteString("\"history\": [\n")
	for i, entry := range c.History {
		line, err := json.Marshal(entry)
		if err != nil {
			return nil, err
		}
		block.WriteString("    ")
		block.Write(line)
		if i < len(c.History)-1 {
			block.WriteByte(',')
		}
		block.WriteByte('\n')
	}
	block.WriteString("  ]")

	// Splice the block in as the last top-level key, before the closing brace.
	closeIdx := bytes.LastIndexByte(body, '}')
	prefix := bytes.TrimRight(body[:closeIdx], " \n\t")

	result := make([]byte, 0, len(body)+block.Len()+8)
	result = append(result, prefix...)
	result = append(result, ",\n  "...)
	result = append(result, block.String()...)
	result = append(result, "\n}"...)
	return result, nil
}

// CurrentColumnSinceMillis returns the event-time at which the card entered its
// current column, derived from history. Falls back to CreatedAtMillis when
// history is empty (defensive; all cards seed a column entry at creation).
func (c Card) CurrentColumnSinceMillis() int64 {
	for i := len(c.History) - 1; i >= 0; i-- {
		if c.History[i].Field == "column" {
			return c.History[i].At
		}
	}
	return c.CreatedAtMillis
}

// reservedCardFieldNames are the built-in card JSON keys. A custom field must
// not use one of these names: it would collide with a built-in property when the
// card is (un)marshaled, silently breaking storage, sorting, and display. This
// is the single source of truth for "built-in field" - UnmarshalJSON uses it to
// separate custom fields from built-ins, and validation uses it to reject
// colliding names up front.
var reservedCardFieldNames = map[string]bool{
	"_v": true, "id": true, "alias": true, "alias_explicit": true,
	"title": true, "description": true,
	"parent": true, "creator": true,
	"created_at_millis": true, "updated_at_millis": true,
	"comments": true, "history": true,
	"column": true, "position": true,
	// Computed/API-only fields that may appear in JSON from external sources
	// (e.g. the restore endpoint) but aren't custom fields.
	"missing_wanted_fields": true,
}

// ValidateCustomFieldName checks if a custom field name is allowed. Returns an
// error if the name uses a reserved prefix or collides with a built-in field.
func ValidateCustomFieldName(name string) error {
	if strings.HasPrefix(name, "_") {
		return &ReservedFieldPrefixError{FieldName: name, Prefix: "_"}
	}
	if strings.HasPrefix(name, "kan_") {
		return &ReservedFieldPrefixError{FieldName: name, Prefix: "kan_"}
	}
	if reservedCardFieldNames[name] {
		return &ReservedFieldNameError{FieldName: name}
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

// ReservedFieldNameError indicates a custom field name collides with a built-in
// card property (e.g. "title", "description", "position"). Such a field would
// be silently dropped or overwritten when the card is (un)marshaled.
type ReservedFieldNameError struct {
	FieldName string
}

func (e *ReservedFieldNameError) Error() string {
	return "custom field \"" + e.FieldName + "\" collides with a built-in card field " +
		"(reserved for Kan internal use). Try \"x_" + e.FieldName + "\" instead."
}
