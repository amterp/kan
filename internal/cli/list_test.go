package cli

import (
	"reflect"
	"testing"
)

// Filter values must reach the matcher whole: a string field's value may hold
// commas, and a set value with a comma gets its own error rather than being
// split silently.
func TestListFilterFlags_Parse(t *testing.T) {
	ctx := buildRootCmd()
	args := []string{"list", "-f", "type=bug", "-f", "owner=Doe, Jane", "-x", "labels=a,b", "--category", "open"}
	if err := ctx.RootCmd.ParseOrError(args); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if want := []string{"type=bug", "owner=Doe, Jane"}; !reflect.DeepEqual(*ctx.ListFilter, want) {
		t.Errorf("filters = %q, want %q", *ctx.ListFilter, want)
	}
	if want := []string{"labels=a,b"}; !reflect.DeepEqual(*ctx.ListExclude, want) {
		t.Errorf("excludes = %q, want %q", *ctx.ListExclude, want)
	}
	if *ctx.ListCategory != "open" {
		t.Errorf("category = %q, want open", *ctx.ListCategory)
	}

	ctx = buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"list", "--category", "urgent"}); err == nil {
		t.Error("expected a parse error for an unknown category")
	}
}
