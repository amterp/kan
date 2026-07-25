package docs_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/api"
	"github.com/amterp/kan/internal/docs"
)

// These run against the real embedded corpus rather than a fake FS, so adding
// or renaming a page under web/src/docs fails here until docs.Order is updated
// to place it.

func TestEmbeddedTopicsMatchOrder(t *testing.T) {
	topics, err := docs.Load(api.DocsFS())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	got := make([]string, 0, len(topics))
	for _, topic := range topics {
		got = append(got, topic.Slug)
	}
	want := append([]string{}, docs.Order...)

	sorted := append([]string{}, got...)
	sort.Strings(sorted)
	sort.Strings(want)
	if strings.Join(sorted, ",") != strings.Join(want, ",") {
		t.Fatalf("embedded slugs = %v, want exactly docs.Order %v", sorted, want)
	}

	// Order also has to be honored, not just the membership.
	if strings.Join(got, ",") != strings.Join(docs.Order, ",") {
		t.Errorf("load order = %v, want %v", got, docs.Order)
	}
}

func TestEmbeddedPagesHaveTitles(t *testing.T) {
	topics, err := docs.Load(api.DocsFS())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	for _, topic := range topics {
		// Load falls back to the slug, so an equal title means no H1 was found.
		if topic.Title == topic.Slug {
			t.Errorf("page %q has no H1 heading", topic.Slug)
		}
		if strings.TrimSpace(topic.Content) == "" {
			t.Errorf("page %q is empty", topic.Slug)
		}
	}
}

func TestEmbeddedPagesDoNotShadowAll(t *testing.T) {
	topics, err := docs.Load(api.DocsFS())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// "all" is reserved by the CLI for printing the whole corpus.
	if _, ok := docs.Find(topics, "all"); ok {
		t.Error("a doc page is named 'all', which the CLI reserves for the full corpus")
	}
}
