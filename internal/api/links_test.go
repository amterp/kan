package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
)

func linkedProject(t *testing.T, id, dataLocation string) *config.Paths {
	t.Helper()
	paths := config.NewPaths(t.TempDir(), dataLocation)
	if err := store.NewBoardStore(paths).Create(&model.BoardConfig{ID: id, Name: "main", DefaultColumn: "done",
		Columns: []model.Column{{Name: "done", Category: model.CategoryDone}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewProjectStore(paths).Save(&model.ProjectConfig{Name: "Remote project"}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewCardStore(paths).Create("main", &model.Card{ID: "shared", Alias: "remote-alias", Title: "Foreign card", Column: "done"}); err != nil {
		t.Fatal(err)
	}
	return paths
}

func projectBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		result[path] = data
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHandler_LinkedCardsReadWithoutSwitchingOrWriting(t *testing.T) {
	remote := linkedProject(t, "b_remote", "board-data")
	api, _ := setupCrossProjectAPI(t, &model.GlobalConfig{Repos: map[string]model.RepoConfig{remote.ProjectRoot(): {DataLocation: "board-data"}}})
	api.createBoard(t, "main")
	source := &model.Card{ID: "shared", Title: "Local source", Column: "backlog", BlockedBy: []string{"b_remote:shared"},
		CustomFields: map[string]any{"ref": "custom ref", "links": "custom links", "result": "custom result", "category": "custom category", "read_only": "custom read_only"}}
	if err := api.cardStore.Create("main", source); err != nil {
		t.Fatal(err)
	}
	if err := api.cardStore.Create("main", &model.Card{ID: "peer", Title: "Local peer", Column: "backlog"}); err != nil {
		t.Fatal(err)
	}
	foreign := store.NewCardStore(remote)
	card, err := foreign.Get("main", "shared")
	if err != nil {
		t.Fatal(err)
	}
	card.BlockedBy = []string{"peer", "test-board-id:shared"}
	if err := foreign.Update("main", card); err != nil {
		t.Fatal(err)
	}
	if err := foreign.Create("main", &model.Card{ID: "peer", Title: "Foreign peer", Column: "done"}); err != nil {
		t.Fatal(err)
	}
	before := projectBytes(t, remote.ProjectRoot())
	original := api.handler.ctx()
	switches := 0
	api.handler.SetOnProjectSwitch(func(string) { switches++ })
	response := api.request("GET", "/api/v1/boards/main/cards", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("list: %d %s", response.Code, response.Body.String())
	}
	var listing struct {
		Cards []CardResponse     `json:"cards"`
		Links BoardLinksResponse `json:"links"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	link := listing.Links.Resolved["b_remote:shared"]
	if link.Result != service.LinkFound || link.Category != model.CategoryDone || link.Card.Title != card.Title || link.ProjectName != "Remote project" || link.ProjectPath != remote.ProjectRoot() {
		t.Fatalf("external link not resolved: %+v", link)
	}
	if len(listing.Links.Blocks["shared"]) != 1 || listing.Links.Blocks["shared"][0].Card.Title != "Foreign card" {
		t.Fatalf("missing external reverse link: %+v", listing.Links)
	}
	var raw struct {
		Cards []map[string]json.RawMessage `json:"cards"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, c := range raw.Cards {
		if bytes.Equal(c["id"], []byte(`"shared"`)) {
			for name, value := range source.CustomFields {
				var got string
				if err := json.Unmarshal(c[name], &got); err != nil || got != value {
					t.Fatalf("metadata overwrote %s: %s", name, c[name])
				}
			}
		}
	}
	response = api.request("GET", "/api/v1/linked-cards/b_remote:shared", nil)
	var view LinkedCardResponse
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || !view.ReadOnly || view.BoardConfig.ID != "b_remote" || view.Links.Resolved["peer"].Card.Title != "Foreign peer" {
		t.Fatalf("read-only view lost target context: %d %s", response.Code, response.Body.String())
	}
	if switches != 0 || api.handler.ctx() != original {
		t.Fatal("reading a link switched server context")
	}
	if !reflect.DeepEqual(before, projectBytes(t, remote.ProjectRoot())) {
		t.Fatal("reading links changed another project")
	}
	for _, method := range []string{"PUT", "POST", "PATCH", "DELETE"} {
		response := api.request(method, "/api/v1/linked-cards/b_remote:shared", map[string]string{"title": "Changed"})
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s read-only endpoint returned %d", method, response.Code)
		}
	}
}

func TestHandler_LinkedCardRejectsDuplicateCurrentBoardIDs(t *testing.T) {
	api, _ := setupCrossProjectAPI(t, &model.GlobalConfig{})
	for _, name := range []string{"main", "duplicate"} {
		api.createBoard(t, name)
		if err := api.cardStore.Create(name, &model.Card{ID: "shared", Title: name, Column: "backlog"}); err != nil {
			t.Fatal(err)
		}
	}
	before := projectBytes(t, api.handler.ctx().ProjectRoot)
	response := api.request("GET", "/api/v1/linked-cards/test-board-id:shared", nil)
	var view LinkedCardResponse
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || view.Result != service.LinkAmbiguousBoard || view.Card != nil || view.BoardConfig != nil || !view.ReadOnly {
		t.Fatalf("duplicate board identity opened a card: %d %s", response.Code, response.Body.String())
	}
	if !reflect.DeepEqual(before, projectBytes(t, api.handler.ctx().ProjectRoot)) {
		t.Fatal("ambiguous lookup changed project files")
	}
}

func TestHandler_LinkedCardLookupStatesAndReadOnlyMigration(t *testing.T) {
	for _, state := range []service.LinkState{service.LinkFound, service.LinkAbsent, service.LinkBoardNotHere, service.LinkAmbiguousBoard, service.LinkUnsupportedSchema, service.LinkUnreadable} {
		t.Run(string(state), func(t *testing.T) {
			remote := linkedProject(t, "b_remote", "")
			cfg := &model.GlobalConfig{Repos: map[string]model.RepoConfig{remote.ProjectRoot(): {}}}
			ref := "b_remote:shared"
			switch state {
			case service.LinkFound:
				data, err := os.ReadFile(remote.CardPath("main", "shared"))
				if err != nil {
					t.Fatal(err)
				}
				data = bytes.Replace(data, []byte(`"_v": 4`), []byte(`"_v": 3`), 1)
				if err := os.WriteFile(remote.CardPath("main", "shared"), data, 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(remote.ProjectConfigPath(), filepath.Join(t.TempDir(), "saved-config.toml")); err != nil {
					t.Fatal(err)
				}
			case service.LinkAbsent:
				ref = "b_remote:missing"
			case service.LinkBoardNotHere:
				cfg.Repos[remote.ProjectRoot()] = model.RepoConfig{Hidden: true}
			case service.LinkAmbiguousBoard:
				clone := linkedProject(t, "b_remote", "")
				cfg.Repos[clone.ProjectRoot()] = model.RepoConfig{}
			case service.LinkUnsupportedSchema:
				if err := os.WriteFile(remote.BoardConfigPath("main"), []byte("kan_schema = \"board/999\"\nid = \"b_remote\"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case service.LinkUnreadable:
				if err := os.WriteFile(remote.CardPath("main", "shared"), []byte("invalid"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			api, _ := setupCrossProjectAPI(t, cfg)
			api.createBoard(t, "main")
			before := projectBytes(t, remote.ProjectRoot())
			response := api.request("GET", "/api/v1/linked-cards/"+ref, nil)
			var view LinkedCardResponse
			if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || view.Result != state || !view.ReadOnly {
				t.Fatalf("lookup state: %d %s", response.Code, response.Body.String())
			}
			if state == service.LinkUnsupportedSchema && view.MinKanVersion == "" {
				t.Fatal("missing required version")
			}
			if !reflect.DeepEqual(before, projectBytes(t, remote.ProjectRoot())) {
				t.Fatal("lookup wrote foreign files")
			}
		})
	}
	api := setupTestAPI(t)
	for _, ref := range []string{"bare", "board:", ":card"} {
		if response := api.request("GET", "/api/v1/linked-cards/"+ref, nil); response.Code != http.StatusBadRequest {
			t.Fatalf("malformed ref %q: %d", ref, response.Code)
		}
	}
}

type delayedLinkRegistry struct {
	*mockGlobalStore
	first   atomic.Bool
	started chan struct{}
	resume  chan struct{}
}

func (s *delayedLinkRegistry) Load() (*model.GlobalConfig, error) {
	if s.first.CompareAndSwap(false, true) {
		close(s.started)
		<-s.resume
	}
	return s.mockGlobalStore.Load()
}

func TestHandler_LinkedCardReadKeepsItsSourceContextDuringExplicitSwitch(t *testing.T) {
	remote := linkedProject(t, "b_remote", "")
	next := linkedProject(t, "b_next", "")
	cfg := &model.GlobalConfig{Repos: map[string]model.RepoConfig{remote.ProjectRoot(): {}, next.ProjectRoot(): {}}}
	api, gs := setupCrossProjectAPI(t, cfg)
	api.createBoard(t, "main")
	delayed := &delayedLinkRegistry{mockGlobalStore: gs, started: make(chan struct{}), resume: make(chan struct{})}
	api.handler.globalStore = delayed
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- api.request("GET", "/api/v1/linked-cards/b_remote:shared", nil) }()
	<-delayed.started
	switched := api.request("POST", "/api/v1/switch", map[string]string{"project_path": next.ProjectRoot()})
	close(delayed.resume)
	if switched.Code != http.StatusOK {
		t.Fatalf("explicit switch failed: %s", switched.Body.String())
	}
	read := <-done
	var view LinkedCardResponse
	if err := json.Unmarshal(read.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if read.Code != http.StatusOK || view.ProjectPath != remote.ProjectRoot() || view.Card.Title != "Foreign card" {
		t.Fatalf("mixed request contexts: %s", read.Body.String())
	}
	project := api.request("GET", "/api/v1/project", nil)
	var current ProjectResponse
	decodeJSON(t, project, &current)
	if current.ProjectPath != next.ProjectRoot() {
		t.Fatalf("read reverted explicit switch: %+v", current)
	}
}

func TestHandler_DeletedParentDoesNotPreventChildSaveOrComment(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")
	parent := createParentTestCard(t, api, "Parent", "")
	child := createParentTestCard(t, api, "Child", parent.ID)
	if deleted := api.request("DELETE", "/api/v1/boards/main/cards/"+parent.ID, nil); deleted.Code != http.StatusNoContent {
		t.Fatalf("delete failed: %s", deleted.Body.String())
	}
	if saved := api.request("PUT", "/api/v1/boards/main/cards/"+child.ID, map[string]string{"title": "Edited child"}); saved.Code != http.StatusOK {
		t.Fatalf("child save failed: %s", saved.Body.String())
	}
	if comment := api.request("POST", "/api/v1/boards/main/cards/"+child.ID+"/comments", map[string]string{"body": "Still editable"}); comment.Code != http.StatusCreated {
		t.Fatalf("child comment failed: %s", comment.Body.String())
	}
	stored, err := api.cardStore.Get("main", child.ID)
	if err != nil || stored.Parent != parent.ID || stored.Title != "Edited child" || len(stored.Comments) != 1 {
		t.Fatalf("child state: %+v, %v", stored, err)
	}
}

func TestHandler_ListCardsLoadsWhenTheRegistryCannotBeRead(t *testing.T) {
	api, gs := setupCrossProjectAPI(t, &model.GlobalConfig{})
	api.createBoard(t, "main")
	gs.err = errors.New("toml: line 3: expected '='")

	resp := api.request("GET", "/api/v1/boards/main/cards", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("an unreadable global config failed the board: %d %s", resp.Code, resp.Body.String())
	}
}
