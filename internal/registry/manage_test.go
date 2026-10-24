package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/model"
)

func TestInspect_States(t *testing.T) {
	useTempRoots(t, "/nonexistent-temp-root")
	ok := makeProject(t, resolved(t), "", "b_ok")
	gone := filepath.Join(resolved(t), "deleted")
	noBoards := resolved(t)
	dupA := makeProject(t, resolved(t), "", "b_dup")
	dupB := makeProject(t, resolved(t), "", "b_dup")
	hiddenDup := makeProject(t, resolved(t), "", "b_ok")

	cfg := &model.GlobalConfig{Repos: map[string]model.RepoConfig{
		ok: {}, gone: {}, noBoards: {}, dupA: {}, dupB: {}, hiddenDup: {Hidden: true},
	}}

	got := map[string]State{}
	for _, s := range Inspect(cfg, true) {
		got[s.Path] = s.State
	}
	want := map[string]State{
		ok:        StateOK,
		gone:      StateMissing,
		noBoards:  StateMissing,
		dupA:      StateDuplicate,
		dupB:      StateDuplicate,
		hiddenDup: StateOK, // hidden entries never count as duplicates
	}
	for path, w := range want {
		if got[path] != w {
			t.Errorf("%s: state %q, want %q", path, got[path], w)
		}
	}

	for _, s := range Inspect(cfg, false) {
		if s.Hidden {
			t.Errorf("Inspect without hidden returned %s", s.Path)
		}
	}
}

func TestInspect_UnreadableIsNotMissing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions don't apply to root")
	}
	parent := resolved(t)
	root := makeProject(t, filepath.Join(parent, "locked", "project"), "", "b_1")
	locked := filepath.Join(parent, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0755) })

	cfg := &model.GlobalConfig{Repos: map[string]model.RepoConfig{root: {}}}
	statuses := Inspect(cfg, false)
	if len(statuses) != 1 || statuses[0].State != StateUnreadable {
		t.Fatalf("want unreadable, got %+v", statuses)
	}
	if removed := Prune(cfg, false); len(removed) != 0 {
		t.Errorf("prune removed an unreadable entry: %v", removed)
	}
	if _, ok := cfg.Repos[root]; !ok {
		t.Error("unreadable entry is gone")
	}
}

func TestPrune_RemovesOnlyMissing(t *testing.T) {
	present := makeProject(t, resolved(t), "", "b_1")
	gone := filepath.Join(resolved(t), "deleted")
	hiddenGone := filepath.Join(resolved(t), "also-deleted")
	cfg := &model.GlobalConfig{Repos: map[string]model.RepoConfig{
		present: {}, gone: {DataLocation: "x"}, hiddenGone: {Hidden: true},
	}}

	dry := Prune(cfg, true)
	if len(dry) != 2 || len(cfg.Repos) != 3 {
		t.Fatalf("dry run: reported %v, repos left %d", dry, len(cfg.Repos))
	}

	removed := Prune(cfg, false)
	if len(removed) != 2 {
		t.Errorf("removed %v", removed)
	}
	if _, ok := cfg.Repos[present]; !ok || len(cfg.Repos) != 1 {
		t.Errorf("repos after prune: %v", cfg.Repos)
	}
}

func TestFind_MatchesSymlinkedPath(t *testing.T) {
	root := makeProject(t, resolved(t), "", "b_1")
	link := filepath.Join(resolved(t), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cfg := &model.GlobalConfig{Repos: map[string]model.RepoConfig{root: {}}}

	key, err := Find(cfg, link)
	if err != nil || key != root {
		t.Errorf("Find(link) = %q, %v; want %q", key, err, root)
	}
	if _, err := Find(cfg, filepath.Join(root, "elsewhere")); err == nil {
		t.Error("Find of an unregistered path should fail")
	}
}

func TestSetHiddenAndSharedBoards(t *testing.T) {
	first := makeProject(t, resolved(t), "", "b_1")
	second := makeProject(t, resolved(t), "", "b_1")
	cfg := &model.GlobalConfig{Repos: map[string]model.RepoConfig{first: {}, second: {Hidden: true}}}

	key, err := SetHidden(cfg, second, false)
	if err != nil || cfg.Repos[second].Hidden {
		t.Fatalf("unhide: %v, %+v", err, cfg.Repos[second])
	}
	shared := SharedBoards(cfg, key)
	if len(shared["b_1"]) != 1 || shared["b_1"][0] != first {
		t.Errorf("SharedBoards = %v, want b_1 held by %s", shared, first)
	}
}

func TestRemove_DropsSettings(t *testing.T) {
	root := makeProject(t, resolved(t), "", "b_1")
	cfg := &model.GlobalConfig{Repos: map[string]model.RepoConfig{root: {DataLocation: "kan-data"}}}
	if _, err := Remove(cfg, root); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("entry still present: %v", cfg.Repos)
	}
}
