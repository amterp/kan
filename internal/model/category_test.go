package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestIsValidCategory(t *testing.T) {
	for _, c := range ValidCategories {
		if !IsValidCategory(c) {
			t.Errorf("IsValidCategory(%q) = false, want true", c)
		}
	}
	if !IsValidCategory("") {
		t.Error("empty category means none and should be valid")
	}
	for _, c := range []string{"backlog", "Open", "todo", " open"} {
		if IsValidCategory(c) {
			t.Errorf("IsValidCategory(%q) = true, want false", c)
		}
	}
}

func TestDefaultColumns_Categories(t *testing.T) {
	want := map[string]string{
		"backlog":     CategoryOpen,
		"next":        CategoryOpen,
		"in-progress": CategoryActive,
		"done":        CategoryDone,
	}
	for _, col := range DefaultColumns() {
		if col.Category != want[col.Name] {
			t.Errorf("default column %q category = %q, want %q", col.Name, col.Category, want[col.Name])
		}
	}
}

func TestValidateColumns_UnknownCategory(t *testing.T) {
	warnings := ValidateColumns([]Column{
		{Name: "backlog", Category: CategoryOpen},
		{Name: "todo", Category: ""},
		{Name: "p0", Category: "urgent"},
	})
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "'p0'") || !strings.Contains(warnings[0], `"urgent"`) {
		t.Errorf("warning should name the column and the bad value: %s", warnings[0])
	}
}

func TestValidateReady(t *testing.T) {
	columns := []Column{
		{Name: "backlog", Category: CategoryOpen},
		{Name: "next", Category: CategoryOpen},
		{Name: "in-progress", Category: CategoryActive},
		{Name: "done", Category: CategoryDone},
		{Name: "ideas"},
	}
	fields := map[string]CustomFieldSchema{
		"labels":  {Type: FieldTypeEnumSet},
		"blocked": {Type: FieldTypeBoolean},
	}
	cases := []struct {
		name  string
		ready ReadyConfig
		want  []string // substrings, one per expected warning, in order
	}{
		{name: "empty", ready: ReadyConfig{}},
		{
			name:  "valid",
			ready: ReadyConfig{Order: []string{"next", "backlog"}, Exclude: []string{"labels=needs-decision"}},
		},
		{name: "unknown column", ready: ReadyConfig{Order: []string{"todo"}}, want: []string{"'todo' names no column"}},
		{
			name:  "listed twice",
			ready: ReadyConfig{Order: []string{"next", "backlog", "next"}},
			want:  []string{"'next' is listed more than once"},
		},
		{
			name:  "not open",
			ready: ReadyConfig{Order: []string{"in-progress", "ideas"}},
			want:  []string{"'in-progress' is not an open column", "'ideas' is not an open column"},
		},
		{
			name:  "bad exclude",
			ready: ReadyConfig{Exclude: []string{"labels", "=x", "labels=ok"}},
			want:  []string{`"labels" has no '='`, `"=x" has an empty key`},
		},
		{
			name:  "exclude names no custom field",
			ready: ReadyConfig{Exclude: []string{"lables=x"}},
			want:  []string{`"lables=x" names no custom field on this board; exclusions match custom fields: blocked, labels`},
		},
		{
			name:  "exclude value its field can never hold",
			ready: ReadyConfig{Exclude: []string{"blocked=yes", "labels=a,b", "blocked=true"}},
			want:  []string{"must be true or false", "Repeat the filter once per value"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateReady(&BoardConfig{Columns: columns, Ready: tc.ready, CustomFields: fields})
			if len(got) != len(tc.want) {
				t.Fatalf("got %d warnings %v, want %d", len(got), got, len(tc.want))
			}
			for i, sub := range tc.want {
				if !strings.Contains(got[i], sub) {
					t.Errorf("warning %d = %q, want it to contain %q", i, got[i], sub)
				}
			}
		})
	}
}

func TestRenameColumn_KeepsReadyOrder(t *testing.T) {
	b := &BoardConfig{
		Columns: []Column{{Name: "backlog"}, {Name: "next"}},
		Ready:   ReadyConfig{Order: []string{"next", "backlog"}},
	}
	if !b.RenameColumn("next", "today") {
		t.Fatal("rename failed")
	}
	if !reflect.DeepEqual(b.Ready.Order, []string{"today", "backlog"}) {
		t.Errorf("ready order = %v, want [today backlog]", b.Ready.Order)
	}
}

func TestRemoveColumn_DropsReadyOrderEntry(t *testing.T) {
	b := &BoardConfig{
		Columns: []Column{{Name: "backlog"}, {Name: "next"}, {Name: "done"}},
		Ready:   ReadyConfig{Order: []string{"next", "backlog"}},
	}
	b.RemoveColumn("next")
	if !reflect.DeepEqual(b.Ready.Order, []string{"backlog"}) {
		t.Errorf("ready order = %v, want [backlog]", b.Ready.Order)
	}
	b.RemoveColumn("backlog")
	if b.Ready.Order != nil {
		t.Errorf("ready order = %v, want nil once its last entry is gone", b.Ready.Order)
	}
}
