package cli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
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
