package cli

import "testing"

// These tests lock the flag-parsing contract for card placement. The
// distinction between "--position 0" and an omitted flag depends on
// ra's Configured() reporting per the invoked subcommand - the most
// fragile part of the feature - so we assert it directly at the parse layer.
// Three commands now register a --position, and so does `column move`, which
// makes the per-subcommand resolution load-bearing rather than incidental.

func TestPlacementFlags_PositionConfigured(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		wantConfigured bool
		wantValue      int
	}{
		{name: "add omitted", args: []string{"add", "Title"}, wantConfigured: false},
		{name: "add position 0", args: []string{"add", "Title", "--position", "0"}, wantConfigured: true, wantValue: 0},
		{name: "add position -2", args: []string{"add", "Title", "--position", "-2"}, wantConfigured: true, wantValue: -2},
		{name: "edit omitted", args: []string{"edit", "card-x"}, wantConfigured: false},
		{name: "edit position 0", args: []string{"edit", "card-x", "--position", "0"}, wantConfigured: true, wantValue: 0},
		{name: "move omitted", args: []string{"move", "card-x", "done"}, wantConfigured: false},
		{name: "move position 0", args: []string{"move", "card-x", "done", "--position", "0"}, wantConfigured: true, wantValue: 0},
		{name: "move position -1", args: []string{"move", "card-x", "done", "--position", "-1"}, wantConfigured: true, wantValue: -1},
		// --top/--bottom must not read as an explicit --position; they are
		// translated later, in resolvePlacement.
		{name: "move top", args: []string{"move", "card-x", "done", "--top"}, wantConfigured: false},
		{name: "move bottom", args: []string{"move", "card-x", "done", "--bottom"}, wantConfigured: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := buildRootCmd()
			if err := ctx.RootCmd.ParseOrError(tc.args); err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if got := ctx.RootCmd.Configured("position"); got != tc.wantConfigured {
				t.Fatalf("Configured(position) = %v, want %v", got, tc.wantConfigured)
			}
			if tc.wantConfigured {
				var val int
				switch tc.args[0] {
				case "add":
					val = *ctx.AddPosition
				case "edit":
					val = *ctx.EditPosition
				case "move":
					val = *ctx.MovePosition
				}
				if val != tc.wantValue {
					t.Fatalf("position value = %d, want %d", val, tc.wantValue)
				}
			}
		})
	}
}

func TestPlacementFlags_MutualExclusion(t *testing.T) {
	cases := [][]string{
		{"add", "Title", "--position", "0", "--after", "card-x"},
		{"add", "Title", "--before", "card-x", "--after", "card-y"},
		{"add", "Title", "--top", "--bottom"},
		{"add", "Title", "--top", "--position", "0"},
		{"edit", "card-x", "--position", "1", "--before", "card-y"},
		{"edit", "card-x", "--top", "--after", "card-y"},
		{"move", "card-x", "done", "--top", "--bottom"},
		{"move", "card-x", "done", "--position", "1", "--before", "card-y"},
		{"move", "card-x", "done", "--bottom", "--after", "card-y"},
	}
	for _, args := range cases {
		ctx := buildRootCmd()
		if err := ctx.RootCmd.ParseOrError(args); err == nil {
			t.Fatalf("expected mutual-exclusion error for args %v, got nil", args)
		}
	}
}

// The column is positional and optional so an anchor can supply it.
func TestPlacementFlags_MoveColumnOptional(t *testing.T) {
	ctx := buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"move", "card-x", "--after", "card-y"}); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if *ctx.MoveColumn != "" {
		t.Fatalf("MoveColumn = %q, want empty", *ctx.MoveColumn)
	}
	if *ctx.MoveAfter != "card-y" {
		t.Fatalf("MoveAfter = %q, want 'card-y'", *ctx.MoveAfter)
	}
}

// resolvePlacement translates --top/--bottom into the explicit indices the
// service understands, and leaves position nil when nothing was requested (so
// the destination column's insert setting decides).
func TestResolvePlacement_TopBottomTranslation(t *testing.T) {
	cases := []struct {
		name      string
		placement cardPlacement
		want      *int
	}{
		{name: "nothing set", placement: cardPlacement{}, want: nil},
		{name: "top", placement: cardPlacement{top: true}, want: intPtrCLI(0)},
		{name: "bottom", placement: cardPlacement{bottom: true}, want: intPtrCLI(-1)},
		{name: "explicit position wins", placement: cardPlacement{position: 3, positionSet: true, top: true}, want: intPtrCLI(3)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _, err := resolvePlacement(nil, "main", "", tc.placement)
			if err != nil {
				t.Fatalf("resolvePlacement failed: %v", err)
			}
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("position = %d, want nil", *got)
			case tc.want != nil && got == nil:
				t.Fatalf("position = nil, want %d", *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Fatalf("position = %d, want %d", *got, *tc.want)
			}
		})
	}
}

func intPtrCLI(i int) *int { return &i }
