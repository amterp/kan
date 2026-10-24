package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/prompt"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

// otherProject copies fixture to a fresh directory, gives its board the ID
// boardID, and returns its root. The fixtures' card is card-abc (alias
// test-card); extraCards adds copies of it under other IDs, aliased
// <id>-alias.
func otherProject(t *testing.T, fixture, boardID string, extraCards ...string) string {
	t.Helper()
	root := copyFixture(t, fixture)
	paths := config.NewPaths(root, "")
	replaceInFile(t, paths.BoardConfigPath("main"), `id = "board-test-123"`, `id = "`+boardID+`"`)

	card, err := os.ReadFile(paths.CardPath("main", "card-abc"))
	if err != nil {
		t.Fatalf("read card: %v", err)
	}
	for _, id := range extraCards {
		data := strings.Replace(string(card), `"card-abc"`, `"`+id+`"`, 1)
		data = strings.Replace(data, `"test-card"`, `"`+id+`-alias"`, 1)
		if err := os.WriteFile(paths.CardPath("main", id), []byte(data), 0644); err != nil {
			t.Fatalf("write card: %v", err)
		}
	}
	return root
}

func replaceInFile(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s has no %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// registerProjects writes a global config listing roots as visible entries
// and hidden as hidden ones.
func registerProjects(t *testing.T, visible []string, hidden ...string) {
	t.Helper()
	cfg := &model.GlobalConfig{}
	for _, root := range visible {
		cfg.SetRepoConfig(root, model.RepoConfig{})
	}
	for _, root := range hidden {
		cfg.SetRepoConfig(root, model.RepoConfig{Hidden: true})
	}
	if err := store.NewGlobalStore().Save(cfg); err != nil {
		t.Fatalf("save global config: %v", err)
	}
}

// setupCrossProject makes a current project (the working directory) and a
// registered project "api" at an older schema, whose board b_api holds
// card-abc, as the current project's board does, and card-xyz. Returns the
// api project's root.
func setupCrossProject(t *testing.T) string {
	t.Helper()
	openFixtureProject(t, "v14")
	api := otherProject(t, "v2", "b_api", "card-xyz")
	registerProjects(t, []string{api})
	return api
}

func newReadApp(t *testing.T) *App {
	t.Helper()
	app, err := NewAppWithOptions(AppOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}
	return app
}

func TestResolveCard_BoardRefReadsOtherProjectWithoutWriting(t *testing.T) {
	api := setupCrossProject(t)
	before := snapshotProject(t, api)
	app := newReadApp(t)

	for _, ref := range []string{"b_api:card-xyz", "b_api:card-xyz-alias"} {
		res, err := app.ResolveCardWithBoard("", ref, false, forRead)
		if err != nil {
			t.Fatalf("%s: %v", ref, err)
		}
		if res.Card.ID != "card-xyz" || res.BoardName != "main" || res.Project.ProjectRoot != api {
			t.Errorf("%s: got card %s on %s in %s", ref, res.Card.ID, res.BoardName, res.Project.ProjectRoot)
		}
		if !res.Project.OtherProject {
			t.Errorf("%s: project should be marked as another project", ref)
		}
	}

	assertProjectUnchanged(t, before, snapshotProject(t, api))
}

func TestResolveCard_BareIDFoundInOtherProject(t *testing.T) {
	api := setupCrossProject(t)
	app := newReadApp(t)

	res, err := app.ResolveCardWithBoard("", "card-xyz", false, forRead)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Project.ProjectRoot != api {
		t.Errorf("want card from %s, got %s", api, res.Project.ProjectRoot)
	}
}

func TestResolveCard_BareIDInBothProjects_CurrentWins(t *testing.T) {
	setupCrossProject(t)
	app := newReadApp(t)

	res, err := app.ResolveCardWithBoard("", "card-abc", false, forRead)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Project != app {
		t.Errorf("want the current project's card, got one from %s", res.Project.ProjectRoot)
	}
}

func TestResolveCard_AliasesStayLocal(t *testing.T) {
	setupCrossProject(t)
	app := newReadApp(t)

	if _, err := app.ResolveCardWithBoard("", "card-xyz-alias", false, forRead); !kanerr.IsNotFound(err) {
		t.Errorf("another project's alias should not resolve, got %v", err)
	}
}

func TestResolveCard_WriteToOtherProjectRefused(t *testing.T) {
	api := setupCrossProject(t)
	before := snapshotProject(t, api)
	app, err := NewAppWithOptions(AppOptions{})
	if err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}

	want := "card card-xyz is on board b_api in " + api + "; run kan from there to change it."
	for _, ref := range []string{"b_api:card-xyz", "b_api:card-xyz-alias", "card-xyz"} {
		_, err := app.ResolveCardWithBoard("", ref, false, forWrite)
		var other *kanerr.OtherProjectError
		if !errors.As(err, &other) || err.Error() != want {
			t.Errorf("%s:\n got %v\nwant %s", ref, err, want)
		}
	}

	assertProjectUnchanged(t, before, snapshotProject(t, api))
}

func TestResolveBoard_ByIDAcrossProjects(t *testing.T) {
	api := setupCrossProject(t)
	app := newReadApp(t)

	project, board, err := app.ResolveBoard("b_api", false, forRead)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if project.ProjectRoot != api || board != "main" {
		t.Errorf("got %s in %s", board, project.ProjectRoot)
	}

	project, board, err = app.ResolveBoard("board-test-123", false, forWrite)
	if err != nil || project != app || board != "main" {
		t.Errorf("the current project's board ID should resolve for a write: %v", err)
	}

	_, _, err = app.ResolveBoard("b_api", false, forWrite)
	want := "board b_api is in " + api + "; run kan from there to change it."
	if err == nil || err.Error() != want {
		t.Errorf("write:\n got %v\nwant %s", err, want)
	}
}

func TestResolveCard_BoardFlagWithBoardRefIsAnError(t *testing.T) {
	setupCrossProject(t)
	app := newReadApp(t)

	if _, err := app.ResolveCardWithBoard("main", "b_api:card-xyz", false, forRead); err == nil {
		t.Error("-b with a board:card reference should fail")
	}
}

func TestResolveCard_DuplicateVisibleBoardIsAmbiguous(t *testing.T) {
	openFixtureProject(t, "v14")
	api := otherProject(t, "v2", "b_api", "card-xyz")
	clone := otherProject(t, "v2", "b_api")
	registerProjects(t, []string{api, clone})

	_, err := newReadApp(t).ResolveCardWithBoard("", "card-xyz", false, forRead)
	var ambig *kanerr.AmbiguousBoardError
	if !errors.As(err, &ambig) {
		t.Fatalf("want an ambiguous board, got %v", err)
	}

	registerProjects(t, []string{api}, clone)
	res, err := newReadApp(t).ResolveCardWithBoard("", "card-xyz", false, forRead)
	if err != nil || res.Project.ProjectRoot != api {
		t.Errorf("hiding the clone should resolve to %s: %v", api, err)
	}
}

func TestResolveBoard_OtherProjectNewerThanKanFails(t *testing.T) {
	openFixtureProject(t, "v14")
	future := otherProject(t, "project_future", "b_future")
	registerProjects(t, []string{future})

	_, _, err := newReadApp(t).ResolveBoard("b_future", false, forRead)
	var schemaErr *version.SchemaVersionError
	if !errors.As(err, &schemaErr) || !strings.Contains(err.Error(), future) {
		t.Errorf("want a schema error naming %s, got %v", future, err)
	}
}

func TestResolveCard_CardOnTwoOtherBoardsIsAnError(t *testing.T) {
	openFixtureProject(t, "v14")
	api := otherProject(t, "v2", "b_api", "card-xyz")
	web := otherProject(t, "v2", "b_web", "card-xyz")
	registerProjects(t, []string{api, web})

	_, err := newReadApp(t).ResolveCardWithBoard("", "card-xyz", false, forRead)
	if err == nil || !strings.Contains(err.Error(), "b_api:card-xyz") || !strings.Contains(err.Error(), "b_web:card-xyz") {
		t.Errorf("want an error listing both refs, got %v", err)
	}
}

// outsideAnyProject registers an "api" project, as setupCrossProject does,
// and makes an empty directory the working directory. Returns api's root.
func outsideAnyProject(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	api := otherProject(t, "v2", "b_api", "card-xyz")
	registerProjects(t, []string{api})
	t.Chdir(resolvedTempDir(t))
	return api
}

func TestOutsideAnyProject_ReadsResolveThroughTheRegistry(t *testing.T) {
	api := outsideAnyProject(t)
	before := snapshotProject(t, api)
	app := newReadApp(t)

	for _, ref := range []string{"b_api:card-xyz", "card-xyz"} {
		res, err := app.ResolveCardWithBoard("", ref, false, forRead)
		if err != nil {
			t.Fatalf("%s: %v", ref, err)
		}
		if res.Project.ProjectRoot != api {
			t.Errorf("%s: got project %s", ref, res.Project.ProjectRoot)
		}
	}
	if _, _, err := app.ResolveBoard("b_api", false, forRead); err != nil {
		t.Errorf("-b with a board ID: %v", err)
	}

	assertProjectUnchanged(t, before, snapshotProject(t, api))
}

func TestOutsideAnyProject_NeedingTheCurrentProjectSaysNotInitialized(t *testing.T) {
	outsideAnyProject(t)
	app := newReadApp(t)

	var notInit *kanerr.NotInitializedError
	if _, _, err := app.ResolveBoard("", false, forRead); !errors.As(err, &notInit) {
		t.Errorf("empty -b: want not initialized, got %v", err)
	}
	if _, err := app.ResolveCardWithBoard("", "no-such-card", false, forRead); !errors.As(err, &notInit) {
		t.Errorf("unknown bare ref: want not initialized, got %v", err)
	}
}

func TestOutsideAnyProject_WriteGetsTheWriteRule(t *testing.T) {
	api := outsideAnyProject(t)
	app, err := NewAppWithOptions(AppOptions{})
	if err != nil {
		t.Fatalf("NewAppWithOptions: %v", err)
	}

	var other *kanerr.OtherProjectError
	if _, err := app.ResolveCardWithBoard("", "card-xyz", false, forWrite); !errors.As(err, &other) || other.Path != api {
		t.Errorf("want the write rule naming %s, got %v", api, err)
	}
}

// A project registered under a symlinked path and discovered under its real
// path is one project, listed once for reverse-link lookups.
func TestElsewhere_ListsCurrentProjectOnceThroughSymlink(t *testing.T) {
	current := openFixtureProject(t, "v15")
	link := filepath.Join(resolvedTempDir(t), "link")
	if err := os.Symlink(current, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	registerProjects(t, []string{link})

	app := newReadApp(t)
	count := 0
	for _, p := range app.elsewhere(app).Projects {
		if registry.SamePath(p.Path, current) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("current project listed %d times", count)
	}
}

// pickPrompter answers every Select with choice.
type pickPrompter struct {
	prompt.NoopPrompter
	choice string
}

func (p *pickPrompter) Select(string, []string) (string, error) { return p.choice, nil }

// Pickers inside another project's board ask through the command's own
// prompter, so they prompt exactly when the command could.
func TestOpenProjectForReading_UsesCallersPrompter(t *testing.T) {
	setupCrossProject(t)
	api := otherProject(t, "v15", "b_api2")
	second := filepath.Join(api, ".kan", "boards", "second")
	if err := os.CopyFS(second, os.DirFS(filepath.Join(api, ".kan", "boards", "main"))); err != nil {
		t.Fatal(err)
	}
	replaceInFile(t, filepath.Join(second, "config.toml"), `id = "b_api2"`, `id = "b_api2_second"`)
	replaceInFile(t, filepath.Join(second, "config.toml"), `name = "main"`, `name = "second"`)

	app := newReadApp(t)
	app.Prompter = &pickPrompter{choice: "second"}
	project, err := app.openProjectForReading(api, "")
	if err != nil {
		t.Fatal(err)
	}
	board, err := project.BoardResolver.Resolve("", true)
	if err != nil || board != "second" {
		t.Errorf("want the caller's prompter to pick \"second\", got %q, %v", board, err)
	}
}
