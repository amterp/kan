package cli

import "testing"

// These tests lock the flag-parsing contract for card placement. The
// distinction between "--position 0" and an omitted flag depends on
// ra's Configured() reporting per the invoked subcommand - the most
// fragile part of the feature - so we assert it directly at the parse layer.

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
				val := *ctx.AddPosition
				if tc.args[0] == "edit" {
					val = *ctx.EditPosition
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
		{"edit", "card-x", "--position", "1", "--before", "card-y"},
	}
	for _, args := range cases {
		ctx := buildRootCmd()
		if err := ctx.RootCmd.ParseOrError(args); err == nil {
			t.Fatalf("expected mutual-exclusion error for args %v, got nil", args)
		}
	}
}
