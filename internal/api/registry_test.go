package api

import (
	"net/http"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
)

func nameProject(t *testing.T, dir, name string) {
	t.Helper()
	if err := store.NewProjectStore(config.NewPaths(dir, "")).Save(&model.ProjectConfig{Name: name}); err != nil {
		t.Fatalf("save project config: %v", err)
	}
}

func TestHandler_ListAllBoards_SkipsHiddenEntries(t *testing.T) {
	visible := createProjectDir(t, "main")
	hidden := createProjectDir(t, "main")

	api, _ := setupCrossProjectAPI(t, &model.GlobalConfig{
		Repos: map[string]model.RepoConfig{visible: {}, hidden: {Hidden: true}},
	})

	w := api.request("GET", "/api/v1/all-boards", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp AllBoardsResponse
	decodeJSON(t, w, &resp)
	if len(resp.Boards) != 1 || resp.Boards[0].ProjectPath != visible {
		t.Errorf("want only the visible project, got %+v", resp.Boards)
	}
}

func TestHandler_ListAllBoards_SameNameShowsPath(t *testing.T) {
	first := createProjectDir(t, "main")
	second := createProjectDir(t, "main")
	nameProject(t, first, "api")
	nameProject(t, second, "api")

	api, _ := setupCrossProjectAPI(t, &model.GlobalConfig{
		Repos: map[string]model.RepoConfig{first: {}, second: {}},
	})

	w := api.request("GET", "/api/v1/all-boards", nil)
	var resp AllBoardsResponse
	decodeJSON(t, w, &resp)
	if len(resp.Boards) != 2 {
		t.Fatalf("want both projects, got %+v", resp.Boards)
	}
	for _, b := range resp.Boards {
		if want := "api (" + util.PrettyPath(b.ProjectPath) + ")"; b.ProjectName != want {
			t.Errorf("name = %q, want %q", b.ProjectName, want)
		}
	}
}

func TestHandler_SwitchProject_HiddenEntry_NotFound(t *testing.T) {
	hidden := createProjectDir(t, "main")
	api, _ := setupCrossProjectAPI(t, &model.GlobalConfig{
		Repos: map[string]model.RepoConfig{hidden: {Hidden: true}},
	})

	w := api.request("POST", "/api/v1/switch", map[string]any{"project_path": hidden})
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for a hidden project, got %d: %s", w.Code, w.Body.String())
	}
}
