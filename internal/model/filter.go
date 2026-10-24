package model

import (
	"fmt"
	"sort"
	"strings"
)

// Filter is one key=value condition on a card field, as written in
// [ready] exclude and in -f/-x flags.
type Filter struct {
	Key   string
	Value string
}

// ParseFilter parses a "key=value" filter. The value may itself contain '=';
// only the first one separates key from value.
func ParseFilter(expr string) (Filter, error) {
	key, value, ok := strings.Cut(expr, "=")
	if !ok {
		return Filter{}, fmt.Errorf("filter %q has no '='; expected key=value", expr)
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return Filter{}, fmt.Errorf("filter %q has an empty key; expected key=value", expr)
	}
	if value == "" {
		return Filter{}, fmt.Errorf("filter %q has an empty value; expected key=value", expr)
	}
	return Filter{Key: key, Value: value}, nil
}

// ParseFilters parses each "key=value" expression, stopping at the first that
// doesn't parse.
func ParseFilters(exprs []string) ([]Filter, error) {
	filters := make([]Filter, 0, len(exprs))
	for _, expr := range exprs {
		f, err := ParseFilter(expr)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, nil
}

// Matches reports whether the card's value for the filter's custom field
// matches the filter value, following the field's type on the board: a set
// field matches when it contains the value, a boolean compares as "true" or
// "false", and anything else compares as a string. A card without the field,
// or on a board that doesn't define it, never matches.
func (f Filter) Matches(card *Card, board *BoardConfig) bool {
	schema, defined := board.CustomFields[f.Key]
	if !defined {
		return false
	}
	value, present := card.CustomFields[f.Key]
	if !present || value == nil {
		return false
	}

	switch schema.Type {
	case FieldTypeEnumSet, FieldTypeFreeSet:
		switch items := value.(type) {
		case []any:
			for _, item := range items {
				if s, ok := item.(string); ok && s == f.Value {
					return true
				}
			}
		case []string:
			for _, s := range items {
				if s == f.Value {
					return true
				}
			}
		}
		return false
	case FieldTypeBoolean:
		b, ok := value.(bool)
		return ok && fmt.Sprint(b) == f.Value
	default:
		switch value.(type) {
		case []any, []string, map[string]any:
			return false
		}
		return fmt.Sprint(value) == f.Value
	}
}

// FilterSet is a card query: a card matches when every Include filter
// matches, no Exclude filter matches, and, when Category is set, the card's
// column has that category.
type FilterSet struct {
	Include  []Filter
	Exclude  []Filter
	Category string
}

// Matches reports whether the card, read against its board's config, passes
// every condition in the set.
func (s FilterSet) Matches(card *Card, board *BoardConfig) bool {
	if s.Category != "" && board.ColumnCategory(card.Column) != s.Category {
		return false
	}
	for _, f := range s.Include {
		if !f.Matches(card, board) {
			return false
		}
	}
	for _, f := range s.Exclude {
		if f.Matches(card, board) {
			return false
		}
	}
	return true
}

// ValidateFilters checks filters against the boards they will run on. A key
// that none of the boards defines as a custom field is an error, since it can
// only be a typo or a built-in field; a key that only some boards define is
// fine and simply never matches on the others. On a board that defines the
// key, the value must suit the field's type.
func ValidateFilters(filters []Filter, boards ...*BoardConfig) error {
	for _, f := range filters {
		defined := false
		for _, board := range boards {
			schema, ok := board.CustomFields[f.Key]
			if !ok {
				continue
			}
			defined = true
			if err := validateFilterValue(f, schema); err != nil {
				return err
			}
		}
		if !defined {
			return unknownFilterFieldError(f.Key, boards)
		}
	}
	return nil
}

func validateFilterValue(f Filter, schema CustomFieldSchema) error {
	switch schema.Type {
	case FieldTypeBoolean:
		if f.Value != "true" && f.Value != "false" {
			return fmt.Errorf("filter %s=%s: %q is a boolean field, so the value must be true or false",
				f.Key, f.Value, f.Key)
		}
	case FieldTypeEnumSet, FieldTypeFreeSet:
		if strings.Contains(f.Value, ",") {
			return fmt.Errorf("filter %s=%s: %q is a set field, whose values never contain a comma, "+
				"so this could never match. Repeat the filter once per value instead",
				f.Key, f.Value, f.Key)
		}
	}
	return nil
}

func unknownFilterFieldError(key string, boards []*BoardConfig) error {
	var hint string
	switch key {
	case "column":
		hint = " To filter by column, use -c/--column."
	case "category":
		hint = " To filter by column category, use --category."
	}

	if len(boards) == 1 {
		return fmt.Errorf("board %q has no custom field %q; filters match custom fields: %s.%s",
			boards[0].Name, key, customFieldList(boards[0]), hint)
	}
	return fmt.Errorf("no board defines a custom field %q; filters match custom fields.%s", key, hint)
}

func customFieldList(board *BoardConfig) string {
	if len(board.CustomFields) == 0 {
		return "(none defined)"
	}
	names := make([]string, 0, len(board.CustomFields))
	for name := range board.CustomFields {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
