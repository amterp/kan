package cli

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
)

func TestReadyFlags_Parse(t *testing.T) {
	ctx := buildRootCmd()
	args := []string{"ready", "-b", "main", "-x", "owner=Doe, Jane", "-x", "labels=urgent", "--no-exclude", "--limit", "3"}
	if err := ctx.RootCmd.ParseOrError(args); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if want := []string{"owner=Doe, Jane", "labels=urgent"}; !reflect.DeepEqual(*ctx.ReadyExclude, want) {
		t.Errorf("excludes = %q, want %q", *ctx.ReadyExclude, want)
	}
	if !*ctx.ReadyNoExclude || *ctx.ReadyLimit != 3 || *ctx.ReadyBoard != "main" {
		t.Errorf("no-exclude = %v, limit = %d, board = %q", *ctx.ReadyNoExclude, *ctx.ReadyLimit, *ctx.ReadyBoard)
	}

	ctx = buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"ready"}); err != nil || *ctx.ReadyLimit != 0 {
		t.Errorf("without --limit: err = %v, limit = %d", err, *ctx.ReadyLimit)
	}

	ctx = buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"ready", "--limit", "0"}); err == nil {
		t.Error("expected a parse error for --limit 0")
	}
}

func TestReadyOutput_JSON(t *testing.T) {
	board := &model.BoardConfig{ID: "b1", Name: "main", Columns: []model.Column{
		{Name: "next", Category: model.CategoryOpen},
		{Name: "done", Category: model.CategoryDone},
		{Name: "ideas"},
	}}
	shipped := &model.Card{ID: "c0", Column: "done"}
	card := &model.Card{
		ID: "c1", Title: "Add kan ready", Column: "next", BlockedBy: []string{"c0", "c9", "c8"},
		CustomFields: map[string]any{"ref": "custom value"},
	}
	result := &service.ReadyResult{
		Board: board,
		Ready: []service.ReadyEntry{{Card: card, Blockers: []service.LinkLookup{
			{State: service.LinkFound, Ref: "c0", Card: shipped, Column: board.GetColumn("done")},
			{State: service.LinkFound, Ref: "c9", Card: &model.Card{ID: "c9", Column: "ideas"}, Column: board.GetColumn("ideas")},
			{State: service.LinkAbsent, Ref: "c8"},
		}}},
	}

	data, err := json.Marshal(NewReadyOutput([]readyBoard{{ProjectPath: "/src/kan", Result: result}}, nil))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"attention", "skipped"} {
		if list, ok := got[key].([]any); !ok || len(list) != 0 {
			t.Errorf("%s = %v, want an empty array", key, got[key])
		}
	}
	entry := got["ready"].([]any)[0].(map[string]any)
	wantContext := map[string]any{
		"ref": "b1:c1", "board_id": "b1", "board": "main", "project_path": "/src/kan", "category": "open",
	}
	for k, v := range wantContext {
		if entry[k] != v {
			t.Errorf("entry[%q] = %v, want %v", k, entry[k], v)
		}
	}
	wantBlockers := []any{
		map[string]any{"ref": "c0", "result": "found", "category": "done"},
		map[string]any{"ref": "c9", "result": "found"},
		map[string]any{"ref": "c8", "result": "absent"},
	}
	if !reflect.DeepEqual(entry["blockers"], wantBlockers) {
		t.Errorf("blockers = %v, want %v", entry["blockers"], wantBlockers)
	}
	inner := entry["card"].(map[string]any)
	if inner["ref"] != "custom value" || inner["id"] != "c1" || inner["board"] != nil {
		t.Errorf("card = %v, want the card's own JSON with its custom ref field", inner)
	}
}

func TestReadyOne_BoardIDOfAnotherProjectFromOutsideAnyProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	api := otherProject(t, "v15", "b_api")
	writeTestCard(t, api, "r1", "Next", "")
	registerProjects(t, []string{api})
	t.Chdir(resolvedTempDir(t))

	got, err := readyOne(newReadApp(t), "b_api", false, service.ReadyOptions{})
	if err != nil {
		t.Fatalf("readyOne: %v", err)
	}
	if got.ProjectPath != api || len(got.Result.Ready) != 1 || got.Result.Ready[0].Card.ID != "r1" {
		t.Errorf("got project %s, ready %+v; want r1 from %s", got.ProjectPath, got.Result.Ready, api)
	}
}

// With -I, two boards and no default must fail with the board error rather
// than open a prompt, which has no terminal under an agent.
func TestReadyOne_NonInteractiveWithSeveralBoards(t *testing.T) {
	addSecondBoard(t, openFixtureProject(t, "v15"))

	app, err := NewAppWithOptions(AppOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = readyOne(app, "", false, service.ReadyOptions{})
	if err == nil || !strings.Contains(err.Error(), "-b") {
		t.Errorf("error = %v, want one asking for -b", err)
	}
}

// addSecondBoard copies a project's main board config to a board named
// second, with its own ID.
func addSecondBoard(t *testing.T, root string) {
	t.Helper()
	paths := config.NewPaths(root, "")
	if err := os.MkdirAll(paths.CardsDir("second"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(paths.BoardConfigPath("main"))
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(strings.Replace(string(data), `name = "main"`, `name = "second"`, 1), `id = "board-test-123"`, `id = "b_second"`, 1))
	if err := os.WriteFile(paths.BoardConfigPath("second"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

// Categorizing has to happen in the board's own project, so the hint for
// another project's board names that project.
func TestReadyOne_OtherProjectWithoutCategoriesNamesTheProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := otherProject(t, "v2", "b_old")
	registerProjects(t, []string{old})
	t.Chdir(resolvedTempDir(t))

	_, err := readyOne(newReadApp(t), "b_old", false, service.ReadyOptions{})
	if err == nil || !strings.Contains(err.Error(), `Run "kan column categorize" in `+old) {
		t.Errorf("error = %v, want the categorize hint naming %s", err, old)
	}
}

func TestReadyOne_CurrentProjectWithoutCategoriesKeepsThePlainHint(t *testing.T) {
	openFixtureProject(t, "v2")
	_, err := readyOne(newReadApp(t), "", false, service.ReadyOptions{})
	if err == nil || !strings.Contains(err.Error(), `Run "kan column categorize" to set them.`) {
		t.Errorf("error = %v, want the plain categorize hint", err)
	}
}
