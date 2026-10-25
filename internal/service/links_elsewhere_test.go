package service

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/store"
)

// linkProject is a project on disk with one board, "main", and a card
// service whose card IDs start with the project's name.
type linkProject struct {
	root    string
	paths   *config.Paths
	boards  store.BoardStore
	cards   store.CardStore
	project store.ProjectStore
	svc     *CardService
}

func newLinkProject(t *testing.T, name, boardID string) *linkProject {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, name)
	paths := config.NewPaths(root, "")
	p := &linkProject{
		root:    root,
		paths:   paths,
		boards:  store.NewBoardStore(paths),
		cards:   store.NewCardStore(paths),
		project: store.NewProjectStore(paths),
	}
	p.addBoard(t, "main", boardID)
	if err := p.project.EnsureInitialized(name); err != nil {
		t.Fatal(err)
	}
	p.svc = NewCardService(p.cards, p.boards, NewAliasService(p.cards))
	next := 0
	p.svc.generateCardID = func() string {
		next++
		return fmt.Sprintf("%s-%d", name, next)
	}
	return p
}

func (p *linkProject) addBoard(t *testing.T, name, boardID string) {
	t.Helper()
	if err := p.boards.Create(&model.BoardConfig{
		ID: boardID, Name: name, DefaultColumn: "open",
		Columns: []model.Column{{Name: "open", Category: model.CategoryOpen}, {Name: "done", Category: model.CategoryDone}},
	}); err != nil {
		t.Fatal(err)
	}
}

func (p *linkProject) add(t *testing.T, title string, blockedBy ...string) *model.Card {
	t.Helper()
	card, _, err := p.svc.Add(AddCardInput{BoardName: "main", Title: title, BlockedBy: blockedBy})
	if err != nil {
		t.Fatal(err)
	}
	return card
}

// setLinks writes links straight into a card, as another kan would have.
func (p *linkProject) setLinks(t *testing.T, card *model.Card, blockedBy ...string) {
	t.Helper()
	card.BlockedBy = blockedBy
	if err := p.cards.Update("main", card); err != nil {
		t.Fatal(err)
	}
}

func (p *linkProject) move(t *testing.T, card *model.Card, column string) {
	t.Helper()
	card.Column = column
	if err := p.cards.Update("main", card); err != nil {
		t.Fatal(err)
	}
}

// registryOf lists projects as visible registry entries.
func registryOf(projects ...*linkProject) *model.GlobalConfig {
	cfg := &model.GlobalConfig{}
	for _, p := range projects {
		cfg.SetRepoConfig(p.root, model.RepoConfig{})
	}
	return cfg
}

// elsewhereFor reaches other projects from p the way the CLI does: through
// the registry, opening each read-only.
func elsewhereFor(p *linkProject, cfg *model.GlobalConfig) *Elsewhere {
	projects := registry.Visible(cfg)
	if !slices.ContainsFunc(projects, func(e registry.Entry) bool { return e.Path == p.root }) {
		projects = append(projects, registry.Entry{Path: p.root})
	}
	return &Elsewhere{
		Locator: registry.NewLocator(cfg, p.paths, p.boards),
		Open: func(root, dataLocation string) (store.BoardStore, store.CardStore, error) {
			paths := config.NewPaths(root, dataLocation)
			overlay, err := NewQuietMigrateService(paths).LoadProjectOverlay()
			if err != nil {
				return nil, nil, fmt.Errorf("reading %s: %w", root, err)
			}
			return store.NewReadOnlyBoardStore(paths, overlay), store.NewReadOnlyCardStore(paths, overlay), nil
		},
		Projects: projects,
		Remote:   func(root string) string { return "https://example.com/" + filepath.Base(root) + ".git" },
	}
}

func (p *linkProject) graph(t *testing.T, cfg *model.GlobalConfig) *LinkGraph {
	t.Helper()
	g, err := LoadLinkGraph(p.boards, p.cards)
	if err != nil {
		t.Fatal(err)
	}
	g.SetElsewhere(elsewhereFor(p, cfg))
	return g
}

func (p *linkProject) linkService(cfg *model.GlobalConfig) *LinkService {
	s := NewLinkService(p.cards, p.boards)
	s.SetElsewhere(elsewhereFor(p, cfg), p.project)
	return s
}

func TestLinkGraph_ResolvesBoardInAnotherProject(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	target := api.add(t, "Endpoint")
	cfg := registryOf(web, api)
	g := web.graph(t, cfg)

	found := g.Resolve("main", "b_api:"+target.ID)
	if found.State != LinkFound || found.Project != api.root || found.BoardName != "main" || found.BoardID != "b_api" {
		t.Fatalf("got %+v", found)
	}
	if found.Column == nil || found.Column.Category != model.CategoryOpen {
		t.Errorf("column = %+v", found.Column)
	}

	if absent := g.Resolve("main", "b_api:api-99"); absent.State != LinkAbsent {
		t.Errorf("missing card: got %s (%s)", absent.State, absent.Reason)
	}
	// Stored refs never match an alias.
	if byAlias := g.Resolve("main", "b_api:"+target.Alias); byAlias.State != LinkAbsent {
		t.Errorf("alias: got %s", byAlias.State)
	}
}

func TestLinkGraph_BoardNotOnThisMachine(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	g := web.graph(t, registryOf(web))
	g.elsewhere.LinkedBoards = map[string]model.LinkedBoard{
		"b_api": {Name: "api/main", Remote: "git@github.com:example/api.git"},
	}

	got := g.Resolve("main", "b_api:a_2n5h6dlsT")
	if got.State != LinkBoardNotHere {
		t.Fatalf("got %s (%s)", got.State, got.Reason)
	}
	want := "a_2nQ0vLx1k waits on api/main (b_api), which isn't on this machine.\n" +
		"One place to get it: git@github.com:example/api.git"
	if msg := NotOnThisMachine("a_2nQ0vLx1k", got); msg != want {
		t.Errorf("message:\n%s\nwant:\n%s", msg, want)
	}

	unrecorded := g.Resolve("main", "b_other:a_1")
	if msg := NotOnThisMachine("a_2nQ0vLx1k", unrecorded); msg != "a_2nQ0vLx1k waits on b_other, which isn't on this machine." {
		t.Errorf("without a record: %s", msg)
	}
}

func TestLinkGraph_DuplicateBoardIsAmbiguous(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	clone := newLinkProject(t, "api-clone", "b_api")
	target := api.add(t, "Endpoint")

	got := web.graph(t, registryOf(web, api, clone)).Resolve("main", "b_api:"+target.ID)
	if got.State != LinkAmbiguousBoard || !strings.Contains(got.Reason, api.root) || !strings.Contains(got.Reason, clone.root) {
		t.Errorf("got %s (%s)", got.State, got.Reason)
	}
}

func TestLinkGraph_NewerProjectIsUnsupported(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	target := api.add(t, "Endpoint")
	if err := os.WriteFile(api.paths.ProjectConfigPath(), []byte("kan_schema = \"project/999\"\nid = \"p_x\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got := web.graph(t, registryOf(web, api)).Resolve("main", "b_api:"+target.ID)
	if got.State != LinkUnsupportedSchema {
		t.Errorf("got %s (%s)", got.State, got.Reason)
	}
}

func TestFindReady_BlockerOnAnotherProject(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	blocker := api.add(t, "Endpoint")
	waiting := web.add(t, "Page")
	web.setLinks(t, waiting, "b_api:"+blocker.ID)
	cfg := registryOf(web, api)

	ready := func() bool {
		result, err := FindReady(web.graph(t, cfg), "main", ReadyOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(result.Ready, func(e ReadyEntry) bool { return e.Card.ID == waiting.ID })
	}
	if ready() {
		t.Error("an open blocker on another project should hold the card back")
	}
	api.move(t, blocker, "done")
	if !ready() {
		t.Error("a done blocker on another project should release the card")
	}
}

func TestLinkService_LinksAcrossBoardsAndRecordsOtherProjects(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	web.addBoard(t, "notes", "b_notes")
	api := newLinkProject(t, "api", "b_api")
	endpoint := api.add(t, "Endpoint")
	own := web.add(t, "Own board")
	note, _, err := web.svc.Add(AddCardInput{BoardName: "notes", Title: "A note"})
	if err != nil {
		t.Fatal(err)
	}
	card := web.add(t, "Page")
	cfg := registryOf(web, api)

	linked, err := web.linkService(cfg).Link("main", card.ID,
		[]string{"b_api:" + endpoint.Alias, "main:" + own.ID}, []string{"notes:" + note.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(linked.BlockedBy, []string{"b_api:" + endpoint.ID, own.ID}) {
		t.Errorf("blocked_by = %v", linked.BlockedBy)
	}
	if !slices.Equal(linked.Related, []string{"b_notes:" + note.ID}) {
		t.Errorf("related = %v", linked.Related)
	}

	projectCfg, err := web.project.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]model.LinkedBoard{"b_api": {Name: "api/main", Remote: "https://example.com/api.git"}}
	if len(projectCfg.LinkedBoards) != 1 || projectCfg.LinkedBoards["b_api"] != want["b_api"] {
		t.Errorf("linked boards = %+v, want only %+v", projectCfg.LinkedBoards, want)
	}
}

func TestCardService_AddBlockedByAnotherProject(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	endpoint := api.add(t, "Endpoint")
	cfg := registryOf(web, api)
	web.svc.SetElsewhere(elsewhereFor(web, cfg), web.project)

	card := web.add(t, "Page", "b_api:"+endpoint.ID)
	if !slices.Equal(card.BlockedBy, []string{"b_api:" + endpoint.ID}) {
		t.Errorf("blocked_by = %v", card.BlockedBy)
	}
	if projectCfg, _ := web.project.Load(); projectCfg.LinkedBoards["b_api"].Name != "api/main" {
		t.Errorf("linked boards = %+v", projectCfg.LinkedBoards)
	}
}

func TestLinkService_RefusesCycleAcrossProjects(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	page := web.add(t, "Page")
	endpoint := api.add(t, "Endpoint")
	api.setLinks(t, endpoint, "b_web:"+page.ID)

	_, err := web.linkService(registryOf(web, api)).Link("main", page.ID, []string{"b_api:" + endpoint.ID}, nil)
	if err == nil || !strings.Contains(err.Error(), "blocker cycle") {
		t.Errorf("want a blocker cycle, got %v", err)
	}
}

func TestLinkService_UnreadableBoardDoesNotBlockALink(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	page := web.add(t, "Page")
	endpoint := api.add(t, "Endpoint")
	api.setLinks(t, endpoint, "b_gone:x")

	if _, err := web.linkService(registryOf(web, api)).Link("main", page.ID, []string{"b_api:" + endpoint.ID}, nil); err != nil {
		t.Errorf("a path past a board that isn't here should not block the link: %v", err)
	}
}

func TestLinkService_RefusesUnknownOrAmbiguousBoards(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	clone := newLinkProject(t, "api-clone", "b_api")
	page := web.add(t, "Page")
	endpoint := api.add(t, "Endpoint")
	svc := web.linkService(registryOf(web, api, clone))

	for _, ref := range []string{"b_nowhere:" + endpoint.ID, "b_api:" + endpoint.ID, "b_api:"} {
		if _, err := svc.Link("main", page.ID, []string{ref}, nil); err == nil {
			t.Errorf("%s: want an error", ref)
		}
	}
	if cfg, _ := web.project.Load(); len(cfg.LinkedBoards) != 0 {
		t.Errorf("a refused link recorded %+v", cfg.LinkedBoards)
	}
}

func TestLinkService_UnlinkByBoardRef(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	page := web.add(t, "Page")
	endpoint := api.add(t, "Endpoint")
	web.setLinks(t, page, "b_api:"+endpoint.ID)

	card, err := web.linkService(registryOf(web, api)).Unlink("main", page.ID, []string{"b_api:" + endpoint.Alias})
	if err != nil {
		t.Fatal(err)
	}
	if len(card.BlockedBy) != 0 {
		t.Errorf("blocked_by = %v", card.BlockedBy)
	}
}

func TestLinkGraph_ReverseLinksFromOtherProjects(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	webClone := newLinkProject(t, "web-clone", "b_web")
	page := web.add(t, "Page")
	endpoint := api.add(t, "Endpoint")
	api.setLinks(t, endpoint, "b_web:"+page.ID)
	cloned := webClone.add(t, "In the clone")
	webClone.setLinks(t, cloned, page.ID)
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.MkdirAll(filepath.Join(gone, ".kan", "boards", "main"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gone, ".kan", "config.toml"), []byte("kan_schema = \"project/999\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := registryOf(web, api, webClone)
	cfg.SetRepoConfig(gone, model.RepoConfig{})

	g := web.graph(t, cfg)
	skipped := g.LoadOtherProjects()
	blocks := g.Blocks("main", page.ID)

	if len(blocks) != 1 || blocks[0].Card.ID != endpoint.ID || blocks[0].Project != api.root || blocks[0].BoardID != "b_api" {
		t.Errorf("blocks = %+v", blocks)
	}
	if len(skipped) != 1 || skipped[0].Path != gone {
		t.Errorf("skipped = %+v", skipped)
	}
	var reasons []string
	for _, s := range skipped {
		reasons = append(reasons, s.Reason)
	}
	if !strings.Contains(strings.Join(reasons, " "), "project/999") {
		t.Errorf("skip reason should name the schema: %v", reasons)
	}
}

// The registry may hold a project under a symlinked spelling while the
// locator knows it by its real path; LoadOtherProjects must still read it.
func TestLinkGraph_ReverseLinksFromProjectRegisteredThroughSymlink(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	endpoint := api.add(t, "Endpoint")
	page := web.add(t, "Page")
	web.setLinks(t, page, "b_api:"+endpoint.ID)
	link := filepath.Join(t.TempDir(), "web-link")
	if err := os.Symlink(web.root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	cfg := &model.GlobalConfig{}
	cfg.SetRepoConfig(link, model.RepoConfig{})
	cfg.SetRepoConfig(api.root, model.RepoConfig{})
	e := elsewhereFor(web, cfg)
	e.Projects = registry.Visible(cfg)

	g, err := LoadLinkGraph(api.boards, api.cards)
	if err != nil {
		t.Fatal(err)
	}
	g.SetElsewhere(e)
	g.LoadOtherProjects()

	if blocks := g.Blocks("main", endpoint.ID); len(blocks) != 1 || blocks[0].Card.ID != page.ID {
		t.Errorf("blocks = %+v", blocks)
	}
}

func TestLinkService_KeepsARecordedRemoteThisMachineCantShare(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	endpoint := api.add(t, "Endpoint")
	cfg := registryOf(web, api)
	if _, err := web.linkService(cfg).Link("main", web.add(t, "Page").ID, []string{"b_api:" + endpoint.ID}, nil); err != nil {
		t.Fatal(err)
	}

	local := web.linkService(cfg)
	local.elsewhere.Remote = func(string) string { return "" }
	if _, err := local.Link("main", web.add(t, "Other").ID, nil, []string{"b_api:" + endpoint.ID}); err != nil {
		t.Fatal(err)
	}
	projectCfg, err := web.project.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := projectCfg.LinkedBoards["b_api"].Remote; got != "https://example.com/api.git" {
		t.Errorf("remote = %q, want the one recorded earlier kept", got)
	}
}
