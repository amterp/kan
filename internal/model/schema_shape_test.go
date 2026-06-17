package model

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/version"
)

// expectedPersistedFields records, per CURRENT schema version, the exact set of
// serialized field paths in the corresponding persisted struct.
//
// This guards against the easiest schema-discipline mistake: adding (or
// removing/renaming) a field on a persisted struct and forgetting to bump the
// schema version. The other schema tests only check that *declared* versions
// are internally consistent, so an unversioned field change sails through them.
//
// When this test fails because you changed a persisted struct, that change is a
// schema change. Per docs/COMPAT.md you must:
//  1. bump the Current<Type>Version constant in internal/version/version.go,
//  2. add a NEW entry here keyed by the new version string with the new shape,
//  3. add migration fixtures + a COMPAT.md note.
//
// Do NOT edit an existing version's entry to silence the failure - that hides
// exactly the unversioned change this test exists to catch.
//
// Limitation: this tracks fields serialized via struct tags only. Fields
// handled by a custom MarshalJSON/UnmarshalJSON are invisible here - notably
// Card.CustomFields (tagged json:"-"), which is flattened into the card JSON by
// hand, so changes to that custom (un)marshaling won't trip this guard.
var expectedPersistedFields = map[string][]string{
	"board/12": {
		"card_display",
		"card_display.badges",
		"card_display.default_sort",
		"card_display.default_sort_desc",
		"card_display.metadata",
		"card_display.tint",
		"card_display.type_indicator",
		"columns",
		"columns.color",
		"columns.description",
		"columns.limit",
		"columns.name",
		"custom_fields",
		"custom_fields.description",
		"custom_fields.options",
		"custom_fields.options.color",
		"custom_fields.options.description",
		"custom_fields.options.value",
		"custom_fields.type",
		"custom_fields.wanted",
		"default_column",
		"id",
		"kan_schema",
		"link_rules",
		"link_rules.name",
		"link_rules.pattern",
		"link_rules.url",
		"name",
		"pattern_hooks",
		"pattern_hooks.command",
		"pattern_hooks.name",
		"pattern_hooks.pattern_title",
		"pattern_hooks.timeout",
	},
	"card/3": {
		"_v",
		"alias",
		"alias_explicit",
		"column",
		"comments",
		"comments.author",
		"comments.body",
		"comments.created_at_millis",
		"comments.id",
		"comments.updated_at_millis",
		"created_at_millis",
		"creator",
		"description",
		"history",
		"history.at",
		"history.field",
		"history.value",
		"id",
		"parent",
		"position",
		"title",
		"updated_at_millis",
	},
	"global/2": {
		"editor",
		"global_board",
		"global_board.board",
		"global_board.path",
		"kan_schema",
		"projects",
		"repos",
		"repos.data_location",
		"repos.default_board",
	},
	"project/2": {
		"favicon",
		"favicon.background",
		"favicon.emoji",
		"favicon.icon_type",
		"favicon.letter",
		"id",
		"kan_schema",
		"name",
		"worktree_independent",
	},
}

func TestPersistedStructShapeIsVersioned(t *testing.T) {
	cases := []struct {
		name    string
		version string
		tagKey  string
		typ     reflect.Type
	}{
		{"BoardConfig", version.CurrentBoardSchema(), "toml", reflect.TypeOf(BoardConfig{})},
		{"GlobalConfig", version.CurrentGlobalSchema(), "toml", reflect.TypeOf(GlobalConfig{})},
		{"ProjectConfig", version.CurrentProjectSchema(), "toml", reflect.TypeOf(ProjectConfig{})},
		{"Card", fmt.Sprintf("card/%d", version.CurrentCardVersion), "json", reflect.TypeOf(Card{})},
	}

	for _, c := range cases {
		want, ok := expectedPersistedFields[c.version]
		if !ok {
			t.Errorf("%s: no recorded field set for schema %q.\n"+
				"If you bumped the schema version, add an entry to expectedPersistedFields "+
				"keyed by %q with the struct's current field set.", c.name, c.version, c.version)
			continue
		}
		got := serializedFieldPaths(c.typ, c.tagKey)
		added, removed := diffSortedSets(want, got)
		if len(added) > 0 || len(removed) > 0 {
			t.Errorf("%s persisted shape changed but its version is still %q.\n"+
				"added (in struct, not recorded):\n%s\n"+
				"removed (recorded, not in struct):\n%s\n"+
				"This is a schema change: bump the version constant, add a NEW "+
				"expectedPersistedFields entry under the new key with this shape, and add "+
				"migration fixtures + a COMPAT.md note. Do not edit the %q entry to silence this.",
				c.name, c.version, formatPaths(added), formatPaths(removed), c.version)
		}
	}
}

// serializedFieldPaths returns the sorted, dotted paths of every exported field
// serialized under tagKey ("toml"/"json"), recursing into nested struct types
// (through pointers, slices, arrays, and map values). Fields tagged "-" are
// skipped; interface-typed fields (e.g. `any`) are leaves.
func serializedFieldPaths(t reflect.Type, tagKey string) []string {
	var out []string

	var walk func(t reflect.Type, prefix string, ancestors map[reflect.Type]bool)
	walk = func(t reflect.Type, prefix string, ancestors map[reflect.Type]bool) {
		t = deref(t)
		if t.Kind() != reflect.Struct || ancestors[t] {
			return
		}
		// Copy the ancestor set so sibling branches can reuse a type; only a
		// type appearing within its own ancestry (a cycle) is pruned.
		next := make(map[reflect.Type]bool, len(ancestors)+1)
		for k := range ancestors {
			next[k] = true
		}
		next[t] = true

		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue // unexported
			}
			name := strings.Split(f.Tag.Get(tagKey), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			out = append(out, path)
			walk(elemStruct(f.Type), path, next)
		}
	}

	walk(t, "", map[reflect.Type]bool{})
	sort.Strings(out)
	return out
}

// elemStruct unwraps pointer/slice/array/map layers to the underlying element
// (or map value) type, so we recurse into nested struct shapes regardless of
// how they're contained. Returns the unwrapped type (may be non-struct).
func elemStruct(t reflect.Type) reflect.Type {
	for {
		switch t.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Array:
			t = t.Elem()
		case reflect.Map:
			t = t.Elem()
		default:
			return t
		}
	}
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

// diffSortedSets returns elements only in got (added) and only in want (removed).
func diffSortedSets(want, got []string) (added, removed []string) {
	w := make(map[string]bool, len(want))
	for _, s := range want {
		w[s] = true
	}
	g := make(map[string]bool, len(got))
	for _, s := range got {
		g[s] = true
	}
	for _, s := range got {
		if !w[s] {
			added = append(added, s)
		}
	}
	for _, s := range want {
		if !g[s] {
			removed = append(removed, s)
		}
	}
	return added, removed
}

func formatPaths(paths []string) string {
	if len(paths) == 0 {
		return "  (none)"
	}
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "\t\t%q,\n", p)
	}
	return b.String()
}
