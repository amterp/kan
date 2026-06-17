package gitdriver_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/git"
	"github.com/amterp/kan/internal/gitdriver"
	"github.com/amterp/kan/internal/model"
)

// These tests drive real git: they build the kan binary, install the merge
// driver into a temp repo, create divergent branches, and merge them - the same
// flow two collaborators hit. They validate the wiring (driver invocation, arg
// parsing, file I/O, exit codes) that pure merge unit tests can't.

// kanBin is the kan binary built once in TestMain and shared by every test - the
// build dominates runtime, so building per-test would multiply it needlessly.
var kanBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "kanbin")
	if err != nil {
		fmt.Fprintln(os.Stderr, "build kan: mktemp:", err)
		os.Exit(1)
	}
	kanBin = filepath.Join(dir, "kan")
	if out, berr := exec.Command("go", "build", "-o", kanBin, "github.com/amterp/kan/cmd/kan").CombinedOutput(); berr != nil {
		fmt.Fprintf(os.Stderr, "build kan: %v\n%s", berr, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func buildKan(t *testing.T) string {
	t.Helper()
	return kanBin
}

func git_(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git_(t, dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func cardsDir(repo string) string {
	return filepath.Join(repo, ".kan", "boards", "main", "cards")
}

func writeCard(t *testing.T, repo string, c model.Card) {
	t.Helper()
	b, err := c.MarshalFile()
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cardsDir(repo), c.ID+".json"), b, 0o644); err != nil {
		t.Fatalf("write card: %v", err)
	}
}

func readCard(t *testing.T, repo, id string) model.Card {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(cardsDir(repo), id+".json"))
	if err != nil {
		t.Fatalf("read card: %v", err)
	}
	var c model.Card
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("card %s is not valid JSON after merge: %v\n%s", id, err, b)
	}
	return c
}

func baseCard() model.Card {
	return model.Card{
		Version:         3,
		ID:              "c_1",
		Alias:           "card",
		Title:           "Title",
		Creator:         "alice",
		CreatedAtMillis: 1000,
		UpdatedAtMillis: 1000,
		Column:          "Backlog",
		Position:        "m",
		History:         []model.HistoryEntry{{Field: "column", Value: "Backlog", At: 1000}},
	}
}

// setupRepo creates a git repo with the merge driver installed and a single
// committed base card, then returns the repo path and the base commit SHA.
func setupRepo(t *testing.T, bin string) (repo, baseSHA string) {
	t.Helper()
	repo = t.TempDir()
	mustGit(t, repo, "init")
	mustGit(t, repo, "config", "user.email", "test@example.com")
	mustGit(t, repo, "config", "user.name", "Test")

	if err := os.MkdirAll(cardsDir(repo), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := gitdriver.Install(git.NewClient(), repo, ".kan", bin); err != nil {
		t.Fatalf("install driver: %v", err)
	}

	writeCard(t, repo, baseCard())
	mustGit(t, repo, "add", "-A")
	mustGit(t, repo, "commit", "-m", "base")
	baseSHA = strings.TrimSpace(mustGit(t, repo, "rev-parse", "HEAD"))
	return repo, baseSHA
}

// mergeBranches applies editA and editB to the base card on two branches off
// baseSHA, then merges b into a. It returns the merge error (nil = clean merge).
func mergeBranches(t *testing.T, repo, baseSHA string, editA, editB func(*model.Card)) error {
	t.Helper()

	mustGit(t, repo, "checkout", "-b", "branch-a", baseSHA)
	a := baseCard()
	editA(&a)
	writeCard(t, repo, a)
	mustGit(t, repo, "commit", "-am", "edit a")

	mustGit(t, repo, "checkout", "-b", "branch-b", baseSHA)
	b := baseCard()
	editB(&b)
	writeCard(t, repo, b)
	mustGit(t, repo, "commit", "-am", "edit b")

	mustGit(t, repo, "checkout", "branch-a")
	_, err := git_(t, repo, "merge", "--no-edit", "branch-b")
	return err
}

func TestIntegration_DifferentFieldsSameCard_AutoMerges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}
	bin := buildKan(t)
	repo, base := setupRepo(t, bin)

	err := mergeBranches(t, repo, base,
		func(c *model.Card) { c.Title = "Title from A"; c.UpdatedAtMillis = 2000 },
		func(c *model.Card) { c.Description = "Desc from B"; c.UpdatedAtMillis = 3000 },
	)
	if err != nil {
		t.Fatalf("expected clean merge, got error: %v", err)
	}

	got := readCard(t, repo, "c_1")
	if got.Title != "Title from A" {
		t.Errorf("title = %q, want %q", got.Title, "Title from A")
	}
	if got.Description != "Desc from B" {
		t.Errorf("description = %q, want %q", got.Description, "Desc from B")
	}
	if got.UpdatedAtMillis != 3000 {
		t.Errorf("updated_at = %d, want max 3000", got.UpdatedAtMillis)
	}
}

func TestIntegration_BothMoveSameCard_LastWriterWins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}
	bin := buildKan(t)
	repo, base := setupRepo(t, bin)

	err := mergeBranches(t, repo, base,
		func(c *model.Card) {
			c.Column = "Doing"
			c.Position = "n"
			c.UpdatedAtMillis = 2000
			c.History = append(c.History, model.HistoryEntry{Field: "column", Value: "Doing", At: 2000})
		},
		func(c *model.Card) {
			c.Column = "Done"
			c.Position = "p"
			c.UpdatedAtMillis = 3000
			c.History = append(c.History, model.HistoryEntry{Field: "column", Value: "Done", At: 3000})
		},
	)
	if err != nil {
		t.Fatalf("expected clean merge, got error: %v", err)
	}

	got := readCard(t, repo, "c_1")
	if got.Column != "Done" {
		t.Errorf("column = %q, want later move Done", got.Column)
	}
	if len(got.History) != 3 {
		t.Errorf("history len = %d, want 3 (both moves unioned with seed): %+v", len(got.History), got.History)
	}
}

func TestIntegration_BothAddComments_Union(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}
	bin := buildKan(t)
	repo, base := setupRepo(t, bin)

	err := mergeBranches(t, repo, base,
		func(c *model.Card) {
			c.UpdatedAtMillis = 2000
			c.Comments = []model.Comment{{ID: "m1", Body: "from A", Author: "alice", CreatedAtMillis: 2000}}
		},
		func(c *model.Card) {
			c.UpdatedAtMillis = 3000
			c.Comments = []model.Comment{{ID: "m2", Body: "from B", Author: "bob", CreatedAtMillis: 3000}}
		},
	)
	if err != nil {
		t.Fatalf("expected clean merge, got error: %v", err)
	}

	got := readCard(t, repo, "c_1")
	if len(got.Comments) != 2 {
		t.Fatalf("comments len = %d, want 2 (union): %+v", len(got.Comments), got.Comments)
	}
}

func TestIntegration_BothEditSameTitle_ConflictSurfaced(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git integration test in -short mode")
	}
	bin := buildKan(t)
	repo, base := setupRepo(t, bin)

	err := mergeBranches(t, repo, base,
		func(c *model.Card) { c.Title = "Title from A"; c.UpdatedAtMillis = 2000 },
		func(c *model.Card) { c.Title = "Title from B"; c.UpdatedAtMillis = 3000 },
	)
	if err == nil {
		t.Fatal("expected merge conflict (non-zero exit), got clean merge")
	}

	// The card file should contain conflict markers and both candidate titles.
	raw, readErr := os.ReadFile(filepath.Join(cardsDir(repo), "c_1.json"))
	if readErr != nil {
		t.Fatalf("read conflicted card: %v", readErr)
	}
	body := string(raw)
	if !strings.Contains(body, "<<<<<<<") || !strings.Contains(body, ">>>>>>>") {
		t.Errorf("expected conflict markers in card:\n%s", body)
	}
	if !strings.Contains(body, "Title from A") || !strings.Contains(body, "Title from B") {
		t.Errorf("expected both candidate titles in conflicted card:\n%s", body)
	}

	// git should report the file as unmerged.
	status := mustGit(t, repo, "status", "--porcelain")
	if !strings.Contains(status, "UU") && !strings.Contains(status, "AA") {
		t.Errorf("expected an unmerged status for the card, got:\n%s", status)
	}
}
