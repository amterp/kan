package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/model"
)

// An empty --category on column edit must clear the category, so presence comes from
// Configured() rather than from a non-empty value.
func TestColumnEditCategory_Configured(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		wantConfigured bool
		wantValue      string
	}{
		{name: "omitted", args: []string{"column", "edit", "next", "--color", "#000000"}, wantConfigured: false},
		{name: "set", args: []string{"column", "edit", "next", "--category", "open"}, wantConfigured: true, wantValue: "open"},
		{name: "cleared", args: []string{"column", "edit", "next", "--category", ""}, wantConfigured: true, wantValue: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := buildRootCmd()
			if err := ctx.RootCmd.ParseOrError(tc.args); err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if got := ctx.RootCmd.Configured("category"); got != tc.wantConfigured {
				t.Fatalf("Configured(category) = %v, want %v", got, tc.wantConfigured)
			}
			if *ctx.ColumnEditCategory != tc.wantValue {
				t.Fatalf("category = %q, want %q", *ctx.ColumnEditCategory, tc.wantValue)
			}
		})
	}
}

func TestColumnAddCategory_RejectsUnknown(t *testing.T) {
	ctx := buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"column", "add", "p0", "--category", "urgent"}); err == nil {
		t.Fatal("expected parse error for an unknown category")
	}

	ctx = buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"column", "add", "p0", "--category", "open"}); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if *ctx.ColumnAddCategory != "open" {
		t.Errorf("category = %q, want open", *ctx.ColumnAddCategory)
	}
}

func TestColumnsOutput_Ready(t *testing.T) {
	columns := []ColumnInfo{{Name: "next", Category: "open"}, {Name: "ideas"}}

	data, err := json.Marshal(NewColumnsOutput(columns, model.ReadyConfig{}))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if strings.Contains(string(data), `"ready"`) {
		t.Errorf("no [ready] settings should mean no ready key: %s", data)
	}
	if !strings.Contains(string(data), `"category":"open"`) || strings.Count(string(data), `"category"`) != 1 {
		t.Errorf("only the categorized column should carry a category: %s", data)
	}

	data, err = json.Marshal(NewColumnsOutput(columns, model.ReadyConfig{Order: []string{"next"}}))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"ready":{"order":["next"]}`) {
		t.Errorf("expected ready order in output: %s", data)
	}
}
