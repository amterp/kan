package docs

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoad_OrdersKnownTopicsThenAppendsRest(t *testing.T) {
	fsys := fstest.MapFS{
		"zebra.md":    &fstest.MapFile{Data: []byte("# Zebra\n")},
		"cli.md":      &fstest.MapFile{Data: []byte("# CLI Reference\n")},
		"aardvark.md": &fstest.MapFile{Data: []byte("# Aardvark\n")},
		"index.md":    &fstest.MapFile{Data: []byte("# Kan Documentation\n")},
		"notes.txt":   &fstest.MapFile{Data: []byte("not markdown")},
	}

	topics, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	want := []string{"index", "cli", "aardvark", "zebra"}
	if len(topics) != len(want) {
		t.Fatalf("got %d topics, want %d", len(topics), len(want))
	}
	for i, slug := range want {
		if topics[i].Slug != slug {
			t.Errorf("topic %d = %q, want %q", i, topics[i].Slug, slug)
		}
	}
}

func TestLoad_TitleFallsBackToSlug(t *testing.T) {
	fsys := fstest.MapFS{
		"cli.md": &fstest.MapFile{Data: []byte("No heading here.\n")},
	}

	topics, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if topics[0].Title != "cli" {
		t.Errorf("Title = %q, want %q", topics[0].Title, "cli")
	}
}

func TestLoad_SkipsHeadingsInsideCodeFences(t *testing.T) {
	// Mirrors the real editing.md (a fenced markdown example) and the shell
	// snippets in configuration.md whose comments start with "# ".
	content := `# Editing Cards

## Keyboard Shortcuts

` + "```bash" + `
# Add a column
kan column add
` + "```" + `

## Supported Markdown

` + "```markdown" + `
## My Card

# Not A Title
` + "```" + `

## Real Last Section
`
	fsys := fstest.MapFS{"editing.md": &fstest.MapFile{Data: []byte(content)}}

	topics, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if topics[0].Title != "Editing Cards" {
		t.Errorf("Title = %q, want %q", topics[0].Title, "Editing Cards")
	}
	want := []string{"Keyboard Shortcuts", "Supported Markdown", "Real Last Section"}
	if strings.Join(topics[0].Sections, "|") != strings.Join(want, "|") {
		t.Errorf("Sections = %v, want %v", topics[0].Sections, want)
	}
}

func TestLoad_TitleIgnoresFencedHeadingBeforeRealOne(t *testing.T) {
	content := "```bash\n# Add a column\n```\n\n# Configuration\n"
	fsys := fstest.MapFS{"configuration.md": &fstest.MapFile{Data: []byte(content)}}

	topics, err := Load(fsys)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if topics[0].Title != "Configuration" {
		t.Errorf("Title = %q, want %q", topics[0].Title, "Configuration")
	}
}

func TestFind(t *testing.T) {
	topics := []Topic{{Slug: "cli"}, {Slug: "index"}}

	if topic, ok := Find(topics, "index"); !ok || topic.Slug != "index" {
		t.Errorf("Find(index) = %v, %v; want index topic", topic, ok)
	}
	if _, ok := Find(topics, "bogus"); ok {
		t.Error("Find(bogus) reported found, want not found")
	}
}

func TestRewriteLinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain internal link",
			in:   "See [Custom Fields](/docs/custom-fields) for full documentation.",
			want: "See Custom Fields (see: kan docs custom-fields) for full documentation.",
		},
		{
			name: "anchor is dropped",
			in:   "[Custom Fields](/docs/custom-fields#card-display) for details.",
			want: "Custom Fields (see: kan docs custom-fields) for details.",
		},
		{
			name: "external link untouched",
			in:   "Kan supports [GFM](https://github.github.com/gfm/) fully.",
			want: "Kan supports [GFM](https://github.github.com/gfm/) fully.",
		},
		{
			name: "same page anchor untouched",
			in:   "Target the global board (see [global](#global))",
			want: "Target the global board (see [global](#global))",
		},
		{
			name: "multiple links on one line",
			in:   "- [CLI](/docs/cli) and [AI Agents](/docs/ai-agents) both apply",
			want: "- CLI (see: kan docs cli) and AI Agents (see: kan docs ai-agents) both apply",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RewriteLinks(tc.in); got != tc.want {
				t.Errorf("RewriteLinks() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderIndex(t *testing.T) {
	topics := []Topic{
		{Slug: "cli", Title: "CLI Reference", Sections: []string{"Commands", "Global Flags"}},
		{Slug: "editing", Title: "Editing Cards"},
	}

	got := RenderIndex(topics)

	for _, want := range []string{
		"Run `kan docs <topic>` to print a page, or `kan docs all` for the full corpus.",
		"- **CLI Reference** (`kan docs cli`): Commands, Global Flags\n",
		"- **Editing Cards** (`kan docs editing`)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderIndex() missing %q, got:\n%s", want, got)
		}
	}
}

func TestRenderAll_IncludesIndexAndRewrittenPages(t *testing.T) {
	topics := []Topic{
		{Slug: "index", Title: "Kan Documentation", Content: "# Kan Documentation\n\nSee [CLI](/docs/cli).\n"},
		{Slug: "cli", Title: "CLI Reference", Content: "# CLI Reference\n"},
	}

	got := RenderAll(topics)

	if !strings.HasPrefix(got, RenderIndex(topics)) {
		t.Errorf("RenderAll() should start with the index, got:\n%s", got)
	}
	if !strings.Contains(got, "See CLI (see: kan docs cli).") {
		t.Errorf("RenderAll() should rewrite internal links, got:\n%s", got)
	}
	if n := strings.Count(got, "\n---\n"); n != len(topics) {
		t.Errorf("RenderAll() has %d separators, want %d", n, len(topics))
	}
}
