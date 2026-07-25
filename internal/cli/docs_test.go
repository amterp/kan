package cli

import (
	"slices"
	"testing"

	"github.com/amterp/kan/internal/docs"
)

// The topic positional is optional, so bare 'docs' and 'docs <topic>' both have
// to parse - and the difference between them drives both the dispatch and the
// --json warning. Assert it at the parse layer.

func TestDocsCommand_ParsesOptionalTopic(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantTopic string
	}{
		{name: "bare", args: []string{"docs"}, wantTopic: ""},
		{name: "topic", args: []string{"docs", "cli"}, wantTopic: "cli"},
		{name: "all", args: []string{"docs", "all"}, wantTopic: "all"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := buildRootCmd()
			if err := ctx.RootCmd.ParseOrError(tc.args); err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			if !*ctx.DocsUsed {
				t.Fatal("DocsUsed = false, want true")
			}
			if *ctx.DocsTopic != tc.wantTopic {
				t.Errorf("DocsTopic = %q, want %q", *ctx.DocsTopic, tc.wantTopic)
			}
		})
	}
}

func TestDocsCommand_NotUsedForOtherCommands(t *testing.T) {
	ctx := buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"list"}); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if *ctx.DocsUsed {
		t.Error("DocsUsed = true for 'list', want false")
	}
}

func TestCompleteDocsTopics(t *testing.T) {
	if got, _ := completeDocsTopics(""); len(got) != len(docs.Order)+1 {
		t.Errorf("completions for empty prefix = %v, want every topic plus 'all'", got)
	}

	got, _ := completeDocsTopics("c")
	want := []string{"custom-fields", "configuration", "cli"}
	if !slices.Equal(got, want) {
		t.Errorf("completions for %q = %v, want %v", "c", got, want)
	}

	got, _ = completeDocsTopics("al")
	if !slices.Equal(got, []string{"all"}) {
		t.Errorf("completions for %q = %v, want [all]", "al", got)
	}
}
