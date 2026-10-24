package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
)

// writeTestCard writes a current-schema card to a project's main board.
func writeTestCard(t *testing.T, root, id, column, extra string) {
	t.Helper()
	data := fmt.Sprintf(`{"_v": 4, "id": %q, "alias": %q, "title": %q, "column": %q, "position": "a",
		"creator": "t", "created_at_millis": 1, "updated_at_millis": 1%s}`, id, id, "Card "+id, column, extra)
	if err := os.WriteFile(config.NewPaths(root, "").CardPath("main", id), []byte(data), 0644); err != nil {
		t.Fatalf("write card: %v", err)
	}
}

func boardKeys(boards []allBoard) []string {
	keys := []string{}
	for _, b := range boards {
		keys = append(keys, b.Project.ProjectRoot+"#"+b.Name)
	}
	return keys
}

func TestBoardsForAll_ProjectsInPathOrderAndSkips(t *testing.T) {
	current := openFixtureProject(t, "v15")
	api := otherProject(t, "v15", "b_api")
	hidden := otherProject(t, "v15", "b_hidden")
	broken := otherProject(t, "v15", "b_broken")
	if err := os.WriteFile(config.NewPaths(broken, "").CardPath("main", "bad"), []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	future := otherProject(t, "project_future", "b_future")
	missing := filepath.Join(resolvedTempDir(t), "gone")
	registerProjects(t, []string{api, broken, future, missing}, hidden)
	snapshots := map[string]map[string][]byte{}
	for _, root := range []string{api, hidden, broken, future} {
		snapshots[root] = snapshotProject(t, root)
	}

	boards, skipped := newReadApp(t).boardsForAll()

	want := []string{current + "#main", api + "#main"}
	sort.Strings(want)
	if got := boardKeys(boards); !reflect.DeepEqual(got, want) {
		t.Errorf("boards = %v, want %v (current included unregistered, hidden left out)", got, want)
	}
	reasons := map[string]string{}
	for _, s := range skipped {
		reasons[s.Path+"#"+s.BoardID] = s.Reason
	}
	if r := reasons[missing+"#"]; !strings.Contains(r, "kan registry prune") {
		t.Errorf("missing project reason = %q", r)
	}
	if r := reasons[broken+"#b_broken"]; !strings.Contains(r, "bad.json") {
		t.Errorf("project with a malformed card: reason = %q, skipped = %+v", r, skipped)
	}
	if r := reasons[future+"#b_future"]; !strings.Contains(r, "newer") && !strings.Contains(r, "upgrade") {
		t.Errorf("newer project reason = %q, skipped = %+v", r, skipped)
	}
	if len(skipped) != 3 {
		t.Errorf("skipped = %+v, want 3 entries", skipped)
	}
	for root, before := range snapshots {
		assertProjectUnchanged(t, before, snapshotProject(t, root))
	}
}

func TestBoardsForAll_CurrentProjectOnceAndHiddenCurrentIncluded(t *testing.T) {
	current := openFixtureProject(t, "v15")
	registerProjects(t, nil, current)

	boards, skipped := newReadApp(t).boardsForAll()
	if got := boardKeys(boards); !reflect.DeepEqual(got, []string{current + "#main"}) || len(skipped) != 0 {
		t.Errorf("boards = %v, skipped = %+v; want only the current project", got, skipped)
	}
	if boards[0].Project.OtherProject {
		t.Error("the current project should use the current App")
	}
}

func TestBoardsForAll_OutsideAnyProject(t *testing.T) {
	api := outsideAnyProject(t)
	boards, _ := newReadApp(t).boardsForAll()
	if got := boardKeys(boards); !reflect.DeepEqual(got, []string{api + "#main"}) {
		t.Errorf("boards = %v, want only the registered project", got)
	}
}

func TestListAll_FlagsApplyWhereTheyMeanSomething(t *testing.T) {
	openFixtureProject(t, "v15")
	qa := otherProject(t, "v15", "b_qa")
	replaceInFile(t, config.NewPaths(qa, "").BoardConfigPath("main"), `name = "Review"`, `name = "QA"`)
	writeTestCard(t, qa, "in-qa", "QA", "")
	appendToFile(t, config.NewPaths(qa, "").BoardConfigPath("main"), "\n[custom_fields.owner]\ntype = \"string\"\n")
	writeTestCard(t, qa, "owned", "Next", `, "owner": "alex"`)
	registerProjects(t, []string{qa})
	app := newReadApp(t)

	cards := func(boards []listedBoard) []string {
		ids := []string{}
		for _, b := range boards {
			for _, c := range b.Cards {
				ids = append(ids, c.ID)
			}
		}
		return ids
	}

	boards, _, err := listAll(app, "", "", []string{"owner=alex"}, nil, "", false)
	if err != nil || !reflect.DeepEqual(cards(boards), []string{"owned"}) {
		t.Errorf("-f on a field one board lacks: %v, %v", cards(boards), err)
	}
	if _, _, err := listAll(app, "", "", []string{"nope=1"}, nil, "", false); err == nil {
		t.Error("expected an error for a field no board defines")
	}

	boards, _, err = listAll(app, "QA", "", nil, nil, "", false)
	if err != nil || !reflect.DeepEqual(cards(boards), []string{"in-qa"}) {
		t.Errorf("-c on a column one board has: %v, %v", cards(boards), err)
	}
	if _, _, err := listAll(app, "Nowhere", "", nil, nil, "", false); err == nil {
		t.Error("expected an error for a column no board has")
	}

	if _, _, err := listAll(app, "", "", nil, nil, model.CategoryDropped, false); err == nil {
		t.Error("expected an error for a category no board has")
	}
	if _, _, err := listAll(app, "", "owner", nil, nil, "", false); err != nil {
		t.Errorf("--sort on a field one board defines: %v", err)
	}
}

func TestListAllOutput_JSON(t *testing.T) {
	board := &model.BoardConfig{ID: "b1", Name: "main", Columns: []model.Column{{Name: "next", Category: model.CategoryOpen}, {Name: "ideas"}}}
	b := listedBoard{
		allBoard: allBoard{Project: &App{ProjectRoot: "/src/kan"}, Name: "main"},
		Config:   board,
		Cards: []*model.Card{
			{ID: "c1", Column: "next", BlockedBy: []string{"c0"}, Related: []string{"b2:c9"}, CustomFields: map[string]any{"ref": "mine"}},
			{ID: "c2", Column: "ideas"},
		},
	}
	data, err := json.Marshal(NewListAllOutput([]listedBoard{b}, nil))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Cards   []map[string]any `json:"cards"`
		Skipped []any            `json:"skipped"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Skipped == nil || len(got.Skipped) != 0 || len(got.Cards) != 2 {
		t.Fatalf("output = %s", data)
	}
	first := got.Cards[0]
	for k, v := range map[string]any{"ref": "b1:c1", "board_id": "b1", "board": "main", "project_path": "/src/kan", "category": "open"} {
		if first[k] != v {
			t.Errorf("entry[%q] = %v, want %v", k, first[k], v)
		}
	}
	card := first["card"].(map[string]any)
	if card["ref"] != "mine" || !reflect.DeepEqual(card["blocked_by"], []any{"c0"}) || !reflect.DeepEqual(card["related"], []any{"b2:c9"}) {
		t.Errorf("card = %v, want its raw links and own ref field", card)
	}
	if _, ok := got.Cards[1]["category"]; ok {
		t.Errorf("a card in an uncategorized column has a category: %v", got.Cards[1])
	}
}

func TestReadyAll_LimitPerBoardAndBoardsWithoutCategoriesSkipped(t *testing.T) {
	current := openFixtureProject(t, "v15")
	api := otherProject(t, "v15", "b_api")
	old := otherProject(t, "v2", "b_old")
	for _, root := range []string{current, api} {
		writeTestCard(t, root, "r1", "Next", "")
		writeTestCard(t, root, "r2", "Next", "")
	}
	registerProjects(t, []string{api, old})

	boards, skipped, err := readyAll(newReadApp(t), service.ReadyOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 2 {
		t.Fatalf("boards = %d, want 2", len(boards))
	}
	for _, b := range boards {
		if len(b.Result.Ready) != 1 {
			t.Errorf("%s: %d ready, want 1 (limit per board)", b.ProjectPath, len(b.Result.Ready))
		}
	}
	if len(skipped) != 1 || skipped[0].Path != old || !strings.Contains(skipped[0].Reason, "no column categories") {
		t.Errorf("skipped = %+v, want the uncategorized board", skipped)
	}
	if _, _, err := readyAll(newReadApp(t), service.ReadyOptions{Exclude: []model.Filter{{Key: "nope", Value: "1"}}}); err == nil {
		t.Error("expected an error for an -x field no board defines")
	}
}

func TestAllFlag_ExcludesBoardAndGlobal(t *testing.T) {
	for _, cmd := range []string{"list", "ready"} {
		for _, flag := range []string{"-b=main", "-g"} {
			ctx := buildRootCmd()
			if err := ctx.RootCmd.ParseOrError([]string{cmd, "--all", flag}); err == nil {
				t.Errorf("%s --all %s: expected a parse error", cmd, flag)
			}
		}
	}
}

func appendToFile(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
