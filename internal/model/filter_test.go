package model

import (
	"strings"
	"testing"
)

func TestParseFilter(t *testing.T) {
	cases := []struct {
		expr    string
		want    Filter
		wantErr bool
	}{
		{expr: "labels=needs-decision", want: Filter{Key: "labels", Value: "needs-decision"}},
		{expr: " type = bug ", want: Filter{Key: "type", Value: "bug"}},
		{expr: "expr=a=b", want: Filter{Key: "expr", Value: "a=b"}},
		{expr: "labels", wantErr: true},
		{expr: "=bug", wantErr: true},
		{expr: "type=", wantErr: true},
		{expr: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := ParseFilter(tc.expr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseFilter(%q) = %+v, want error", tc.expr, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFilter(%q) error: %v", tc.expr, err)
			}
			if got != tc.want {
				t.Errorf("ParseFilter(%q) = %+v, want %+v", tc.expr, got, tc.want)
			}
		})
	}
}

func filterTestBoard() *BoardConfig {
	return &BoardConfig{
		Name: "main",
		Columns: []Column{
			{Name: "backlog", Category: CategoryOpen},
			{Name: "doing", Category: CategoryActive},
			{Name: "ideas"},
		},
		CustomFields: map[string]CustomFieldSchema{
			"type":    {Type: FieldTypeEnum},
			"labels":  {Type: FieldTypeEnumSet},
			"topics":  {Type: FieldTypeFreeSet},
			"blocked": {Type: FieldTypeBoolean},
			"owner":   {Type: FieldTypeString},
			"due":     {Type: FieldTypeDate},
		},
	}
}

func filterTestCard(column string, fields map[string]any) *Card {
	return &Card{ID: "c", Column: column, CustomFields: fields}
}

func TestFilter_Matches(t *testing.T) {
	board := filterTestBoard()
	card := filterTestCard("backlog", map[string]any{
		"type":    "bug",
		"labels":  []any{"needs-decision", "urgent"},
		"topics":  []string{"cli"},
		"blocked": false,
		"owner":   "Alex",
		"due":     "2026-11-01",
		"stray":   "x", // a value whose field the board no longer defines
	})
	cases := []struct {
		filter string
		want   bool
	}{
		{"type=bug", true},
		{"type=feature", false},
		{"labels=urgent", true},
		{"labels=needs-decision", true},
		{"labels=urg", false},
		{"topics=cli", true},
		{"blocked=false", true},
		{"blocked=true", false},
		{"owner=Alex", true},
		{"owner=alex", false}, // strings compare exactly
		{"due=2026-11-01", true},
		{"stray=x", false},   // not defined on the board
		{"missing=1", false}, // neither defined nor present
	}
	for _, tc := range cases {
		t.Run(tc.filter, func(t *testing.T) {
			f, err := ParseFilter(tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			if got := f.Matches(card, board); got != tc.want {
				t.Errorf("Matches = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilter_MatchesNeverWithoutTheField(t *testing.T) {
	board := filterTestBoard()
	bare := filterTestCard("backlog", nil)
	for _, expr := range []string{"type=bug", "labels=urgent", "blocked=false", "owner=Alex"} {
		f, _ := ParseFilter(expr)
		if f.Matches(bare, board) {
			t.Errorf("%s matched a card without the field", expr)
		}
	}

	// A stored value of the wrong shape for its field type never matches.
	odd := filterTestCard("backlog", map[string]any{"blocked": "false", "labels": "urgent", "owner": []any{"Alex"}})
	for _, expr := range []string{"blocked=false", "labels=urgent", "owner=Alex"} {
		f, _ := ParseFilter(expr)
		if f.Matches(odd, board) {
			t.Errorf("%s matched a wrongly shaped value", expr)
		}
	}
}

func TestFilterSet_Matches(t *testing.T) {
	board := filterTestBoard()
	both := filterTestCard("backlog", map[string]any{"labels": []any{"a", "b"}, "type": "bug"})
	onlyA := filterTestCard("doing", map[string]any{"labels": []any{"a"}})
	none := filterTestCard("ideas", nil)

	mustParse := func(exprs ...string) []Filter {
		fs, err := ParseFilters(exprs)
		if err != nil {
			t.Fatal(err)
		}
		return fs
	}
	cases := []struct {
		name string
		set  FilterSet
		want []bool // both, onlyA, none
	}{
		{"empty set keeps everything", FilterSet{}, []bool{true, true, true}},
		{"two includes on one field must both hold", FilterSet{Include: mustParse("labels=a", "labels=b")}, []bool{true, false, false}},
		{"exclude keeps cards without the field", FilterSet{Exclude: mustParse("labels=b")}, []bool{false, true, true}},
		{"include and exclude", FilterSet{Include: mustParse("labels=a"), Exclude: mustParse("type=bug")}, []bool{false, true, false}},
		{"category", FilterSet{Category: CategoryOpen}, []bool{true, false, false}},
		{"uncategorized column never matches a category", FilterSet{Category: CategoryActive}, []bool{false, true, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i, card := range []*Card{both, onlyA, none} {
				if got := tc.set.Matches(card, board); got != tc.want[i] {
					t.Errorf("card %d: Matches = %v, want %v", i, got, tc.want[i])
				}
			}
		})
	}
}

func TestParseFilters(t *testing.T) {
	fs, err := ParseFilters([]string{"a=1", "b=x,y"})
	if err != nil || len(fs) != 2 || fs[1] != (Filter{Key: "b", Value: "x,y"}) {
		t.Fatalf("ParseFilters = %+v, %v", fs, err)
	}
	if _, err := ParseFilters([]string{"a=1", "oops"}); err == nil {
		t.Error("expected an error for an entry without '='")
	}
}

func TestValidateFilters(t *testing.T) {
	main := filterTestBoard()
	other := &BoardConfig{Name: "other", CustomFields: map[string]CustomFieldSchema{
		"size":   {Type: FieldTypeEnum},
		"labels": {Type: FieldTypeFreeSet},
	}}
	cases := []struct {
		name    string
		filter  string
		boards  []*BoardConfig
		wantErr string // substring; empty means valid
	}{
		{"defined", "type=bug", []*BoardConfig{main}, ""},
		{"enum values are not checked against options", "type=whatever", []*BoardConfig{main}, ""},
		{"unknown key lists the fields", "lables=x", []*BoardConfig{main}, `no custom field "lables"; filters match custom fields: blocked, due, labels, owner, topics, type`},
		{"column hint", "column=next", []*BoardConfig{main}, "use -c/--column"},
		{"category hint", "category=open", []*BoardConfig{main}, "use --category"},
		{"defined on one of two boards", "size=m", []*BoardConfig{main, other}, ""},
		{"defined on none of two boards", "nope=1", []*BoardConfig{main, other}, `no board defines a custom field "nope"`},
		{"boolean value", "blocked=yes", []*BoardConfig{main}, "must be true or false"},
		{"comma in a set value", "labels=a,b", []*BoardConfig{main}, "Repeat the filter once per value"},
		{"comma in a string value is fine", "owner=Doe, Jane", []*BoardConfig{main}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := ParseFilter(tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateFilters([]Filter{f}, tc.boards...)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
