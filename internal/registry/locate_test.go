package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

// addCard writes an empty card file named id on board b0 of root.
func addCard(t *testing.T, root, id string) {
	t.Helper()
	path := config.NewPaths(root, "").CardPath("b0", id)
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("write card: %v", err)
	}
}

// makeCurrentProject is makeProject at the current board schema, so the
// current project's store, which validates schemas, can read it.
func makeCurrentProject(t *testing.T, root string, boardIDs ...string) string {
	t.Helper()
	makeProject(t, root, "", boardIDs...)
	for i := range boardIDs {
		path := config.NewPaths(root, "").BoardConfigPath("b" + string(rune('0'+i)))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read board: %v", err)
		}
		data = []byte(strings.Replace(string(data), "board/2", version.CurrentBoardSchema(), 1))
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("write board: %v", err)
		}
	}
	return root
}

func locatorFor(cfg *model.GlobalConfig, current string) *Locator {
	paths := config.NewPaths(current, "")
	return NewLocator(cfg, paths, store.NewBoardStore(paths))
}

func register(cfg *model.GlobalConfig, root string, hidden bool) {
	cfg.SetRepoConfig(root, model.RepoConfig{Hidden: hidden})
}

func TestLocatorBoard_CurrentProjectWinsOverRegistryCopy(t *testing.T) {
	current := makeCurrentProject(t, resolved(t), "b_1")
	clone := makeProject(t, resolved(t), "", "b_1")
	other := makeProject(t, resolved(t), "", "b_1")
	cfg := &model.GlobalConfig{}
	register(cfg, clone, false)
	register(cfg, other, false)

	loc, err := locatorFor(cfg, current).Board("b_1")
	if err != nil {
		t.Fatalf("Board: %v", err)
	}
	if loc.Root != current || loc.Board != "b0" {
		t.Errorf("want the current project's board, got %+v", loc)
	}
}

func TestLocatorBoard_OneVisibleHolder(t *testing.T) {
	current := makeCurrentProject(t, resolved(t), "b_1")
	api := makeProject(t, resolved(t), "", "b_2")
	cfg := &model.GlobalConfig{}
	register(cfg, current, false)
	register(cfg, api, false)

	loc, err := locatorFor(cfg, current).Board("b_2")
	if err != nil {
		t.Fatalf("Board: %v", err)
	}
	if loc.Root != api || loc.Board != "b0" || loc.BoardID != "b_2" {
		t.Errorf("got %+v", loc)
	}
}

func TestLocatorBoard_IncompleteExternalIdentitiesCannotEstablishUniqueness(t *testing.T) {
	for _, contents := range []string{"invalid = [", "name = \"unknown\"\n"} {
		t.Run(contents, func(t *testing.T) {
			current := makeCurrentProject(t, resolved(t), "b_local")
			known := makeProject(t, resolved(t), "", "b_api")
			unknown := makeProject(t, resolved(t), "", "b_api")
			if err := os.WriteFile(config.NewPaths(unknown, "").BoardConfigPath("b0"), []byte(contents), 0644); err != nil {
				t.Fatal(err)
			}
			cfg := &model.GlobalConfig{}
			register(cfg, known, false)
			register(cfg, unknown, false)
			strict := locatorFor(cfg, current)
			strict.RequireKnownIdentities()
			if _, err := strict.Board("b_api"); err == nil || kanerr.IsNotFound(err) || !strings.Contains(err.Error(), unknown) {
				t.Fatalf("incomplete registry established uniqueness: %v", err)
			}
			if loc, err := locatorFor(cfg, current).Board("b_api"); err != nil || loc.Root != known {
				t.Fatalf("a read lookup should name the holder it can identify: %+v, %v", loc, err)
			}
			if loc, err := locatorFor(cfg, current).Board("b_local"); err != nil || loc.Root != current {
				t.Fatalf("external uncertainty hid current board: %+v, %v", loc, err)
			}
		})
	}
}

func TestLocatorBoard_HiddenEntriesNeverCount(t *testing.T) {
	api := makeProject(t, resolved(t), "", "b_2")
	hiddenClone := makeProject(t, resolved(t), "", "b_2")
	cfg := &model.GlobalConfig{}
	register(cfg, api, false)
	register(cfg, hiddenClone, true)

	loc, err := locatorFor(cfg, "").Board("b_2")
	if err != nil || loc.Root != api {
		t.Errorf("want %s, got %+v, %v", api, loc, err)
	}

	register(cfg, api, true)
	if _, err := locatorFor(cfg, "").Board("b_2"); !kanerr.IsNotFound(err) {
		t.Errorf("only hidden holders left: want not found, got %v", err)
	}
}

func TestLocatorBoard_TwoVisibleHoldersAreAmbiguous(t *testing.T) {
	first := makeProject(t, resolved(t), "", "b_2")
	second := makeProject(t, resolved(t), "", "b_2")
	cfg := &model.GlobalConfig{}
	register(cfg, first, false)
	register(cfg, second, false)

	_, err := locatorFor(cfg, "").Board("b_2")
	var ambig *kanerr.AmbiguousBoardError
	if !errors.As(err, &ambig) {
		t.Fatalf("want AmbiguousBoardError, got %v", err)
	}
	if !strings.Contains(err.Error(), first) || !strings.Contains(err.Error(), second) {
		t.Errorf("error should name both paths: %v", err)
	}
}

func TestLocatorBoard_NotFound(t *testing.T) {
	current := makeCurrentProject(t, resolved(t), "b_1")
	if _, err := locatorFor(&model.GlobalConfig{}, current).Board("b_9"); !kanerr.IsNotFound(err) {
		t.Errorf("want not found, got %v", err)
	}
}

func TestLocatorFindCard(t *testing.T) {
	current := makeCurrentProject(t, resolved(t), "b_1")
	api := makeProject(t, resolved(t), "", "b_2")
	cloneOfCurrent := makeProject(t, resolved(t), "", "b_1")
	hidden := makeProject(t, resolved(t), "", "b_3")
	addCard(t, api, "a_1")
	addCard(t, cloneOfCurrent, "a_1")
	addCard(t, hidden, "a_1")
	cfg := &model.GlobalConfig{}
	register(cfg, api, false)
	register(cfg, cloneOfCurrent, false)
	register(cfg, hidden, true)

	found, err := locatorFor(cfg, current).FindCard("a_1")
	if err != nil {
		t.Fatalf("FindCard: %v", err)
	}
	if len(found) != 1 || found[0].Root != api || found[0].BoardID != "b_2" {
		t.Errorf("want only the api board, got %+v", found)
	}
}

func TestLocatorFindCard_IgnoresIDsThatAreNotFileNames(t *testing.T) {
	api := makeProject(t, resolved(t), "", "b_2")
	cfg := &model.GlobalConfig{}
	register(cfg, api, false)
	// A file one level above the cards folder must never match.
	if err := os.WriteFile(filepath.Join(api, ".kan", "boards", "b0", "x.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, id := range []string{"../x", "..", ".", ""} {
		if found, _ := locatorFor(cfg, "").FindCard(id); len(found) != 0 {
			t.Errorf("FindCard(%q) = %+v, want nothing", id, found)
		}
	}
}
