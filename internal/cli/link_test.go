package cli

import (
	"slices"
	"testing"
)

func TestLinkFlags(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		blocked []string
		related []string
	}{
		{args: []string{"link", "source", "--blocked-by", "a", "b", "--related", "c", "d"}, blocked: []string{"a", "b"}, related: []string{"c", "d"}},
		{args: []string{"link", "source", "--blocked-by", "a", "--blocked-by", "b"}, blocked: []string{"a", "b"}},
		{args: []string{"link", "source", "--related", "a", "--related", "b"}, related: []string{"a", "b"}},
	} {
		ctx := buildRootCmd()
		if err := ctx.RootCmd.ParseOrError(tc.args); err != nil {
			t.Fatalf("parse %v: %v", tc.args, err)
		}
		if !*ctx.LinkUsed || *ctx.LinkCard != "source" || !slices.Equal(*ctx.LinkBlockedBy, tc.blocked) || !slices.Equal(*ctx.LinkRelated, tc.related) {
			t.Fatalf("parsed links differ for %v: %v, %v", tc.args, *ctx.LinkBlockedBy, *ctx.LinkRelated)
		}
	}
}

func TestUnlinkAndAddBlockerFlags(t *testing.T) {
	ctx := buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"unlink", "source", "a", "remote:gone", "-b", "main"}); err != nil {
		t.Fatal(err)
	}
	if !*ctx.UnlinkUsed || *ctx.UnlinkCard != "source" || *ctx.UnlinkBoard != "main" || !slices.Equal(*ctx.UnlinkRefs, []string{"a", "remote:gone"}) {
		t.Fatal("unlink flags lost refs or board")
	}
	ctx = buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"add", "Title", "--blocked-by", "a", "b"}); err != nil {
		t.Fatal(err)
	}
	if *ctx.AddTitle != "Title" || !slices.Equal(*ctx.AddBlockedBy, []string{"a", "b"}) {
		t.Fatal("add blocker flag changed title or lost refs")
	}
}
