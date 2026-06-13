package model

import (
	"reflect"
	"testing"
)

// sortTestBoard returns a board config covering each sortable field type.
// priority options are intentionally ordered low-importance -> high-importance
// so option order differs from alphabetical, exercising enum rank ordering.
func sortTestBoard() *BoardConfig {
	return &BoardConfig{
		CustomFields: map[string]CustomFieldSchema{
			"priority": {
				Type: FieldTypeEnum,
				Options: []CustomFieldOption{
					{Value: "ultra-low"},
					{Value: "low"},
					{Value: "medium"},
					{Value: "high"},
				},
			},
			"labels": {
				Type: FieldTypeEnumSet,
				Options: []CustomFieldOption{
					{Value: "blocked"},
					{Value: "needs-review"},
				},
			},
			"assignee": {Type: FieldTypeString},
			"due":      {Type: FieldTypeDate},
			"blocked":  {Type: FieldTypeBoolean},
			"topics":   {Type: FieldTypeFreeSet},
		},
	}
}

// mkCard builds a card with a position and custom fields for sort tests.
func mkCard(id, position string, fields map[string]any) *Card {
	return &Card{ID: id, Position: position, CustomFields: fields}
}

func cardIDs(cards []*Card) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = c.ID
	}
	return out
}

func TestSortCardsByField_EnumOptionOrder(t *testing.T) {
	// Manual order (by position) is descending priority: high, medium, low,
	// ultra-low — exactly how the sample board is arranged.
	build := func() []*Card {
		return []*Card{
			mkCard("high", "A", map[string]any{"priority": "high"}),
			mkCard("medium", "B", map[string]any{"priority": "medium"}),
			mkCard("low", "C", map[string]any{"priority": "low"}),
			mkCard("ultra", "D", map[string]any{"priority": "ultra-low"}),
		}
	}

	t.Run("ascending follows config option order", func(t *testing.T) {
		cards := build()
		SortCardsByField(cards, sortTestBoard(), "priority", false)
		want := []string{"ultra", "low", "medium", "high"}
		if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
			t.Errorf("ascending: got %v, want %v", got, want)
		}
	})

	t.Run("descending reverses", func(t *testing.T) {
		cards := build()
		SortCardsByField(cards, sortTestBoard(), "priority", true)
		want := []string{"high", "medium", "low", "ultra"}
		if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
			t.Errorf("descending: got %v, want %v", got, want)
		}
	})
}

func TestSortCardsByField_UnsetAlwaysLast(t *testing.T) {
	build := func() []*Card {
		return []*Card{
			mkCard("none1", "A", nil),
			mkCard("high", "B", map[string]any{"priority": "high"}),
			mkCard("none2", "C", map[string]any{"priority": ""}), // empty string == unset
			mkCard("low", "D", map[string]any{"priority": "low"}),
		}
	}

	// Unset cards trail the valued ones in both directions; among themselves
	// they keep manual (position) order.
	t.Run("ascending", func(t *testing.T) {
		cards := build()
		SortCardsByField(cards, sortTestBoard(), "priority", false)
		want := []string{"low", "high", "none1", "none2"}
		if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("descending", func(t *testing.T) {
		cards := build()
		SortCardsByField(cards, sortTestBoard(), "priority", true)
		want := []string{"high", "low", "none1", "none2"}
		if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestSortCardsByField_StableTiebreakByPosition(t *testing.T) {
	// Two cards share a value; their relative order must follow manual position
	// (ascending) regardless of sort direction.
	build := func() []*Card {
		return []*Card{
			mkCard("h_late", "B", map[string]any{"priority": "high"}),
			mkCard("h_early", "A", map[string]any{"priority": "high"}),
			mkCard("low", "C", map[string]any{"priority": "low"}),
		}
	}

	t.Run("ascending keeps equal-value cards in position order", func(t *testing.T) {
		cards := build()
		SortCardsByField(cards, sortTestBoard(), "priority", false)
		want := []string{"low", "h_early", "h_late"}
		if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("descending keeps equal-value cards in position order", func(t *testing.T) {
		cards := build()
		SortCardsByField(cards, sortTestBoard(), "priority", true)
		want := []string{"h_early", "h_late", "low"}
		if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestSortCardsByField_EnumSetByMinOption(t *testing.T) {
	// JSON-sourced sets arrive as []any; mirror that here.
	cards := []*Card{
		mkCard("review", "A", map[string]any{"labels": []any{"needs-review"}}),
		mkCard("blocked", "B", map[string]any{"labels": []any{"blocked", "needs-review"}}),
		mkCard("empty", "C", map[string]any{"labels": []any{}}),
	}
	SortCardsByField(cards, sortTestBoard(), "labels", false)
	want := []string{"blocked", "review", "empty"}
	if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSortCardsByField_Boolean(t *testing.T) {
	cards := []*Card{
		mkCard("true1", "A", map[string]any{"blocked": true}),
		mkCard("false1", "B", map[string]any{"blocked": false}),
		mkCard("unset", "C", nil),
	}
	SortCardsByField(cards, sortTestBoard(), "blocked", false)
	// false before true; explicit false outranks unset.
	want := []string{"false1", "true1", "unset"}
	if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSortCardsByField_StringCaseInsensitive(t *testing.T) {
	cards := []*Card{
		mkCard("bob", "A", map[string]any{"assignee": "bob"}),
		mkCard("alice", "B", map[string]any{"assignee": "Alice"}),
		mkCard("carol", "C", map[string]any{"assignee": "carol"}),
	}
	SortCardsByField(cards, sortTestBoard(), "assignee", false)
	want := []string{"alice", "bob", "carol"}
	if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSortCardsByField_Date(t *testing.T) {
	cards := []*Card{
		mkCard("mar", "A", map[string]any{"due": "2026-03-01"}),
		mkCard("jan", "B", map[string]any{"due": "2026-01-15"}),
		mkCard("feb", "C", map[string]any{"due": "2026-02-20"}),
	}
	SortCardsByField(cards, sortTestBoard(), "due", false)
	want := []string{"jan", "feb", "mar"}
	if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSortCardsByField_UnknownEnumValueAfterKnown(t *testing.T) {
	cards := []*Card{
		mkCard("weird", "A", map[string]any{"priority": "weird"}),
		mkCard("high", "B", map[string]any{"priority": "high"}),
		mkCard("ultra", "C", map[string]any{"priority": "ultra-low"}),
	}
	SortCardsByField(cards, sortTestBoard(), "priority", false)
	// Unknown values rank after all defined options but are still "set" (before
	// truly-unset cards, of which there are none here).
	want := []string{"ultra", "high", "weird"}
	if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSortCardsByField_FreeSetByMinValue(t *testing.T) {
	cards := []*Card{
		mkCard("mango", "A", map[string]any{"topics": []any{"mango"}}),
		mkCard("apple", "B", map[string]any{"topics": []any{"zebra", "apple"}}),
	}
	SortCardsByField(cards, sortTestBoard(), "topics", false)
	want := []string{"apple", "mango"}
	if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSortCardsByField_EmptyFieldIsNoOp(t *testing.T) {
	cards := []*Card{
		mkCard("c", "C", nil),
		mkCard("a", "A", nil),
		mkCard("b", "B", nil),
	}
	SortCardsByField(cards, sortTestBoard(), "", false)
	want := []string{"c", "a", "b"} // unchanged
	if got := cardIDs(cards); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
