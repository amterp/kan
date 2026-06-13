package model

import (
	"sort"
	"strings"
)

// SortCardsByField sorts cards in place by the named custom field, using the
// board schema to interpret values. It is a non-destructive view sort: it never
// touches the cards' Position, only their order in the slice.
//
// Ordering rules by field type:
//   - enum: by the option order defined in the board config (the first option
//     ranks lowest). Values not present in the options list rank after all
//     known options, ordered alphabetically among themselves.
//   - enum-set: by the card's highest-ranked (lowest option index) member, so a
//     card tagged with an early option sorts before one tagged only with later
//     options. Ties broken by the sorted, joined values.
//   - free-set: by the card's alphabetically-first member.
//   - string / date: case-insensitive lexicographic, then case-sensitive as a
//     tiebreak. ISO-8601 dates (YYYY-MM-DD) sort chronologically as strings.
//   - boolean: false before true.
//
// Cards with no value for the field always sort to the end, regardless of
// direction—"unset" stays out of the way whether ascending or descending. Cards
// that compare equal (including all unset cards) keep their existing manual
// order (Position, then ID as a final tiebreak), so toggling a sort never
// scrambles the relative order of same-valued cards.
//
// When the field is not defined in the board schema, values are compared by
// their underlying JSON type using the same string/boolean/set rules.
func SortCardsByField(cards []*Card, board *BoardConfig, field string, descending bool) {
	if field == "" {
		return
	}

	var schema CustomFieldSchema
	if board != nil {
		schema = board.CustomFields[field] // zero value (empty Type) if undefined
	}
	cmp := fieldValueComparator(schema)

	sort.SliceStable(cards, func(i, j int) bool {
		ci, cj := cards[i], cards[j]
		vi, hasI := fieldSortValue(ci, field)
		vj, hasJ := fieldSortValue(cj, field)

		// Present values always sort before missing ones, in both directions.
		if hasI != hasJ {
			return hasI
		}

		if hasI && hasJ {
			if c := cmp(vi, vj); c != 0 {
				if descending {
					return c > 0
				}
				return c < 0
			}
		}

		// Equal values (or both unset): preserve manual order.
		if ci.Position != cj.Position {
			return ci.Position < cj.Position
		}
		return ci.ID < cj.ID
	})
}

// fieldSortValue returns a card's raw value for a field and whether it counts as
// "set". Empty strings and empty sets are treated as unset; a boolean is set
// whenever present (false is a meaningful value).
func fieldSortValue(card *Card, field string) (any, bool) {
	v, ok := card.CustomFields[field]
	if !ok || v == nil {
		return nil, false
	}
	switch val := v.(type) {
	case string:
		return val, val != ""
	case []any:
		return val, len(val) > 0
	case []string:
		return val, len(val) > 0
	default:
		return val, true
	}
}

// fieldValueComparator returns a 3-way comparison function for two set values of
// the given field. Both inputs are assumed non-empty (see fieldSortValue).
func fieldValueComparator(schema CustomFieldSchema) func(a, b any) int {
	switch schema.Type {
	case FieldTypeEnum:
		order := optionRanks(schema)
		return func(a, b any) int {
			return compareEnum(asString(a), asString(b), order, len(schema.Options))
		}
	case FieldTypeEnumSet:
		order := optionRanks(schema)
		n := len(schema.Options)
		return func(a, b any) int {
			ra, rb := minOptionRank(a, order, n), minOptionRank(b, order, n)
			if ra != rb {
				return ra - rb
			}
			return strings.Compare(joinSortedSet(a), joinSortedSet(b))
		}
	case FieldTypeBoolean:
		return func(a, b any) int {
			ba, bb := asBool(a), asBool(b)
			switch {
			case ba == bb:
				return 0
			case !ba: // false sorts before true
				return -1
			default:
				return 1
			}
		}
	case FieldTypeFreeSet:
		return func(a, b any) int {
			return compareStrings(minSetValue(a), minSetValue(b))
		}
	default: // string, date, or unknown
		return func(a, b any) int {
			return compareStrings(asString(a), asString(b))
		}
	}
}

// optionRanks maps each enum option value to its index in the config order.
func optionRanks(schema CustomFieldSchema) map[string]int {
	ranks := make(map[string]int, len(schema.Options))
	for i, opt := range schema.Options {
		ranks[opt.Value] = i
	}
	return ranks
}

// compareEnum orders two enum values by their option rank. Values not in the
// options list rank after all known options (at index unknownRank) and are then
// ordered alphabetically so unknown values remain deterministic.
func compareEnum(a, b string, ranks map[string]int, unknownRank int) int {
	ra, oka := ranks[a]
	rb, okb := ranks[b]
	if !oka {
		ra = unknownRank
	}
	if !okb {
		rb = unknownRank
	}
	if ra != rb {
		return ra - rb
	}
	return strings.Compare(a, b)
}

// minOptionRank returns the lowest option rank among a set's values. Values not
// in the options list contribute unknownRank.
func minOptionRank(v any, ranks map[string]int, unknownRank int) int {
	best := unknownRank + 1 // sentinel above any real or unknown rank
	for _, s := range asStringSlice(v) {
		r, ok := ranks[s]
		if !ok {
			r = unknownRank
		}
		if r < best {
			best = r
		}
	}
	return best
}

// minSetValue returns the alphabetically-first member of a set, or "".
func minSetValue(v any) string {
	var min string
	first := true
	for _, s := range asStringSlice(v) {
		if first || s < min {
			min = s
			first = false
		}
	}
	return min
}

// joinSortedSet returns the set's values sorted and joined, for a stable tiebreak.
func joinSortedSet(v any) string {
	vals := append([]string(nil), asStringSlice(v)...)
	sort.Strings(vals)
	return strings.Join(vals, "\x00")
}

// compareStrings compares case-insensitively first, then case-sensitively so
// that values differing only in case are still ordered deterministically.
func compareStrings(a, b string) int {
	if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// asStringSlice coerces a set value (stored as []any from JSON, or []string in
// memory) to a []string, dropping non-string elements.
func asStringSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}
