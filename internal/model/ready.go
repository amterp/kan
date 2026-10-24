package model

import "fmt"

// HasCategories reports whether any column has a category. A board without
// any is invisible to kan ready.
func (b *BoardConfig) HasCategories() bool {
	for _, col := range b.Columns {
		if col.Category != "" {
			return true
		}
	}
	return false
}

// ReadyColumns returns the board's open columns in the order kan ready reads
// them: the columns named in [ready] order first, then the other open columns
// from right to left.
func (b *BoardConfig) ReadyColumns() []Column {
	var result []Column
	listed := make(map[string]bool, len(b.Ready.Order))
	for _, name := range b.Ready.Order {
		col := b.GetColumn(name)
		if col == nil || col.Category != CategoryOpen || listed[name] {
			continue
		}
		listed[name] = true
		result = append(result, *col)
	}
	for i := len(b.Columns) - 1; i >= 0; i-- {
		col := b.Columns[i]
		if col.Category == CategoryOpen && !listed[col.Name] {
			result = append(result, col)
		}
	}
	return result
}

// ReadyExclusions returns the [ready] exclude filters kan ready applies,
// leaving out the entries ValidateReady warns about.
func (b *BoardConfig) ReadyExclusions() []Filter {
	var filters []Filter
	for _, expr := range b.Ready.Exclude {
		if f, err := b.readyExclusion(expr); err == nil {
			filters = append(filters, f)
		}
	}
	return filters
}

func (b *BoardConfig) readyExclusion(expr string) (Filter, error) {
	f, err := ParseFilter(expr)
	if err != nil {
		return Filter{}, err
	}
	schema, ok := b.CustomFields[f.Key]
	if !ok {
		return Filter{}, fmt.Errorf("filter %q names no custom field on this board; "+
			"exclusions match custom fields: %s", expr, customFieldList(b))
	}
	if err := validateFilterValue(f, schema); err != nil {
		return Filter{}, err
	}
	return f, nil
}
