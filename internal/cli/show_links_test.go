package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// captureStdout returns what fn prints to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = saved }()
	fn()
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// setBlockedBy rewrites a fixture card's blocked_by.
func setBlockedBy(t *testing.T, root, cardID string, refs ...string) {
	t.Helper()
	path := config.NewPaths(root, "").CardPath("main", cardID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var card map[string]any
	if err := json.Unmarshal(data, &card); err != nil {
		t.Fatal(err)
	}
	card["blocked_by"] = refs
	if data, err = json.Marshal(card); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestShow_LinksAcrossProjects(t *testing.T) {
	current := openFixtureProject(t, "v15")
	api := otherProject(t, "v15", "b_api", "card-xyz")
	registerProjects(t, []string{api})
	setBlockedBy(t, current, "card-abc", "b_api:card-xyz", "b_gone:a_1")
	setBlockedBy(t, api, "card-abc", "board-test-123:card-abc")

	ps := store.NewProjectStore(config.NewPaths(current, ""))
	cfg, err := ps.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.LinkedBoards = map[string]model.LinkedBoard{"b_gone": {Name: "gone/main", Remote: "https://example.com/gone.git"}}
	if err := ps.Save(cfg); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() { runShow("card-abc", "", false, false, false) })

	for _, want := range []string{
		"b_api:card-xyz  Test Card",
		"card-abc waits on gone/main (b_gone), which isn't on this machine.",
		"One place to get it: https://example.com/gone.git",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	blocks := out[strings.Index(out, "Blocks:"):]
	if !strings.Contains(blocks, "b_api:card-abc") {
		t.Errorf("Blocks should list the other project's card:\n%s", out)
	}
}
