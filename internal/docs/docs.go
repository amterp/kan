// Package docs turns the documentation markdown embedded in the binary into
// topics that `kan docs` can print. It is deliberately stdlib-only and takes
// the corpus as an fs.FS, so it can be exercised without the embed.
package docs

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

// Topic is one documentation page.
type Topic struct {
	Slug     string
	Title    string   // first H1, falling back to the slug
	Sections []string // H2s in document order
	Content  string   // raw markdown
}

// Order lists topics in the same sequence as the web docs nav
// (web/src/pages/DocsPage.tsx), so terminal readers get the reading order the
// site was designed around rather than an alphabetical jumble.
var Order = []string{
	"index",
	"shortcuts",
	"editing",
	"custom-fields",
	"configuration",
	"link-rules",
	"cli",
	"ai-agents",
}

// Load reads every markdown file at the root of fsys. Topics named in Order
// come first; anything else is appended alphabetically so a newly added page is
// still reachable before someone remembers to order it.
func Load(fsys fs.FS) ([]Topic, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}

	bySlug := make(map[string]Topic, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		slug := strings.TrimSuffix(name, ".md")
		title, sections := scanHeadings(string(content))
		if title == "" {
			title = slug
		}
		bySlug[slug] = Topic{Slug: slug, Title: title, Sections: sections, Content: string(content)}
	}

	topics := make([]Topic, 0, len(bySlug))
	for _, slug := range Order {
		if topic, ok := bySlug[slug]; ok {
			topics = append(topics, topic)
			delete(bySlug, slug)
		}
	}

	unordered := make([]string, 0, len(bySlug))
	for slug := range bySlug {
		unordered = append(unordered, slug)
	}
	sort.Strings(unordered)
	for _, slug := range unordered {
		topics = append(topics, bySlug[slug])
	}

	return topics, nil
}

// Find returns the topic with the given slug.
func Find(topics []Topic, slug string) (Topic, bool) {
	for _, topic := range topics {
		if topic.Slug == slug {
			return topic, true
		}
	}
	return Topic{}, false
}

// internalLink matches a link into the web docs, with or without an anchor,
// e.g. [Custom Fields](/docs/custom-fields#card-display).
var internalLink = regexp.MustCompile(`\[([^\]]*)\]\(/docs/([A-Za-z0-9-]+)(?:#[^)]*)?\)`)

// RewriteLinks replaces site-relative doc links with the command that prints
// the linked page, since a terminal reader cannot follow a URL path. Anchors
// are dropped because `kan docs` prints whole pages. External links are left
// untouched.
func RewriteLinks(md string) string {
	return internalLink.ReplaceAllString(md, "$1 (see: kan docs $2)")
}

// RenderIndex builds the listing printed by a bare `kan docs`: how to read a
// page, then one entry per topic with its sections, so a reader can tell which
// page answers their question without printing them all.
func RenderIndex(topics []Topic) string {
	var b strings.Builder
	b.WriteString("# Kan Documentation\n\n")
	b.WriteString("Run `kan docs <topic>` to print a page, or `kan docs all` for the full corpus.\n\n")
	for _, topic := range topics {
		fmt.Fprintf(&b, "- **%s** (`kan docs %s`)", topic.Title, topic.Slug)
		if len(topic.Sections) > 0 {
			fmt.Fprintf(&b, ": %s", strings.Join(topic.Sections, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// RenderAll concatenates the index and every page, rule-separated, for readers
// (usually agents) that want the whole corpus in one pass.
func RenderAll(topics []Topic) string {
	var b strings.Builder
	b.WriteString(RenderIndex(topics))
	for _, topic := range topics {
		b.WriteString("\n---\n\n")
		b.WriteString(strings.TrimSpace(RewriteLinks(topic.Content)))
		b.WriteString("\n")
	}
	return b.String()
}

// scanHeadings extracts the first H1 as the title and every H2 as a section.
// It tracks fenced code blocks and ignores headings inside them: the pages
// embed markdown examples (editing.md has a fenced "## My Card") and shell and
// TOML snippets whose comments start with "# ".
func scanHeadings(content string) (title string, sections []string) {
	inFence := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		switch {
		case title == "" && strings.HasPrefix(line, "# "):
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		case strings.HasPrefix(line, "## "):
			sections = append(sections, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
		}
	}
	return title, sections
}
