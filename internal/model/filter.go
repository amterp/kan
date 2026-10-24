package model

import (
	"fmt"
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
