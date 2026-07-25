package cli

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/api"
	"github.com/amterp/kan/internal/docs"
	"github.com/amterp/ra"
)

// docsTopicInfo describes one topic in the JSON index.
type docsTopicInfo struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Sections []string `json:"sections"`
}

// DocsIndexOutput wraps the topic index for JSON output.
type DocsIndexOutput struct {
	Topics []docsTopicInfo `json:"topics"`
}

// registerDocs adds the "kan docs [topic]" command. It deliberately never
// builds an App: the docs are baked into the binary, so they stay readable
// outside any kan project and without triggering project discovery.
func registerDocs(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("docs")
	cmd.SetDescription("Print the documentation embedded in this binary")

	ctx.DocsTopic, _ = ra.NewString("topic").
		SetOptional(true).
		SetUsage("Topic to print, or 'all' (omit to list topics)").
		SetCompletionFunc(completeDocsTopics).
		Register(cmd)

	ctx.DocsUsed, _ = parent.RegisterCmd(cmd)
}

// completeDocsTopics offers the known topics plus "all". The corpus ships with
// the binary, so unlike board and card completion this needs no project.
func completeDocsTopics(toComplete string) ([]string, ra.CompletionDirective) {
	var result []string
	for _, slug := range docs.Order {
		if strings.HasPrefix(slug, toComplete) {
			result = append(result, slug)
		}
	}
	if strings.HasPrefix("all", toComplete) {
		result = append(result, "all")
	}
	return result, ra.CompletionDirectiveNoFileComp
}

// runDocs prints the topic index, one page, or the whole corpus.
func runDocs(topic string, jsonOut bool) {
	topics, err := docs.Load(api.DocsFS())
	if err != nil {
		Fatal(fmt.Errorf("failed to read embedded docs: %w", err))
	}

	switch topic {
	case "":
		if jsonOut {
			if err := printJson(buildDocsIndex(topics)); err != nil {
				Fatal(err)
			}
			return
		}
		fmt.Print(docs.RenderIndex(topics))

	case "all":
		fmt.Print(docs.RenderAll(topics))

	default:
		found, ok := docs.Find(topics, topic)
		if !ok {
			Fatal(fmt.Errorf("unknown docs topic %q; run 'kan docs' to list topics", topic))
		}
		fmt.Println(strings.TrimSpace(docs.RewriteLinks(found.Content)))
	}
}

func buildDocsIndex(topics []docs.Topic) DocsIndexOutput {
	out := DocsIndexOutput{Topics: make([]docsTopicInfo, 0, len(topics))}
	for _, topic := range topics {
		sections := topic.Sections
		if sections == nil {
			sections = []string{}
		}
		out.Topics = append(out.Topics, docsTopicInfo{Slug: topic.Slug, Title: topic.Title, Sections: sections})
	}
	return out
}
