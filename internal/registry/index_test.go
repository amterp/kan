package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/model"
)

// countReads counts how often a project's boards are read for the rest of
// the test.
func countReads(t *testing.T) *int {
	t.Helper()
	n := 0
	saved := readBoards
	readBoards = func(root, dataLocation string) ([]Board, error) {
		n++
		return saved(root, dataLocation)
	}
	t.Cleanup(func() { readBoards = saved })
	return &n
}

// Queries across projects read each project once, however many boards they
// ask about; reading every project per question made kan show and --all
// quadratic in the number of registered projects.
func TestQueriesReadEachProjectOnce(t *testing.T) {
	const n = 20
	cfg := &model.GlobalConfig{}
	var ids []string
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("b_%d", i)
		ids = append(ids, id)
		cfg.SetRepoConfig(makeProject(t, resolved(t), "", id), model.RepoConfig{})
	}

	reads := countReads(t)
	l := locatorFor(cfg, "")
	for _, id := range ids {
		if _, err := l.Board(id); err != nil {
			t.Fatalf("Board(%s): %v", id, err)
		}
	}
	if _, err := l.FindCard("a_1"); err != nil {
		t.Fatal(err)
	}
	if *reads != n {
		t.Errorf("Locator read projects %d times, want %d", *reads, n)
	}

	*reads = 0
	Inspect(cfg, false)
	if *reads != n {
		t.Errorf("Inspect read projects %d times, want %d", *reads, n)
	}
}

// Two entries for one directory, one through a symlink, are one project:
// its boards aren't ambiguous and it isn't its own duplicate.
func TestSymlinkedEntriesAreOneProject(t *testing.T) {
	target := makeProject(t, resolved(t), "", "b_1")
	link := filepath.Join(resolved(t), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cfg := &model.GlobalConfig{}
	cfg.SetRepoConfig(target, model.RepoConfig{})
	cfg.SetRepoConfig(link, model.RepoConfig{})

	if _, err := locatorFor(cfg, "").Board("b_1"); err != nil {
		t.Errorf("Board: %v", err)
	}
	for _, s := range Inspect(cfg, false) {
		if s.State != StateOK || len(s.Duplicates) != 0 {
			t.Errorf("%s: state %s, duplicates %v", s.Path, s.State, s.Duplicates)
		}
	}
}
