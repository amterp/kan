package model

import (
	"reflect"
	"testing"
)

func columnNames(cols []Column) []string {
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		names = append(names, c.Name)
	}
	return names
}

func TestReadyColumns(t *testing.T) {
	columns := []Column{
		{Name: "backlog-lo", Category: CategoryOpen},
		{Name: "backlog-hi", Category: CategoryOpen},
		{Name: "next", Category: CategoryOpen},
		{Name: "doing", Category: CategoryActive},
		{Name: "ideas"},
	}
	cases := []struct {
		name  string
		order []string
		want  []string
	}{
		{"right to left without an order", nil, []string{"next", "backlog-hi", "backlog-lo"}},
		{"order first, the rest right to left", []string{"backlog-lo"}, []string{"backlog-lo", "next", "backlog-hi"}},
		{
			"unknown, repeated and non-open entries are skipped",
			[]string{"todo", "backlog-hi", "doing", "ideas", "backlog-hi"},
			[]string{"backlog-hi", "next", "backlog-lo"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			board := &BoardConfig{Columns: columns, Ready: ReadyConfig{Order: tc.order}}
			if got := columnNames(board.ReadyColumns()); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ReadyColumns = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReadyExclusions_KeepsOnlyEntriesThatValidate(t *testing.T) {
	board := &BoardConfig{
		CustomFields: map[string]CustomFieldSchema{
			"labels":  {Type: FieldTypeEnumSet},
			"blocked": {Type: FieldTypeBoolean},
		},
		Ready: ReadyConfig{Exclude: []string{
			"labels=needs-decision", "lables=x", "nokey", "blocked=yes", "labels=a,b", "blocked=true",
		}},
	}
	want := []Filter{{Key: "labels", Value: "needs-decision"}, {Key: "blocked", Value: "true"}}
	if got := board.ReadyExclusions(); !reflect.DeepEqual(got, want) {
		t.Errorf("ReadyExclusions = %+v, want %+v", got, want)
	}
}

func TestSortCardsByPosition(t *testing.T) {
	cards := []*Card{
		{ID: "c", Position: "b"},
		{ID: "b", Position: "a"},
		{ID: "a", Position: "b"},
	}
	SortCardsByPosition(cards)
	got := []string{cards[0].ID, cards[1].ID, cards[2].ID}
	if want := []string{"b", "a", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}
