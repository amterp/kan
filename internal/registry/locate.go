package registry

import (
	"os"
	"path/filepath"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// Location is a board in a project on this machine.
type Location struct {
	Root         string
	DataLocation string
	// Board is the board's folder name in that project.
	Board   string
	BoardID string
}

// Locator finds boards and cards by ID across the current project and the
// visible registry entries. The current project's own boards always win,
// whatever else in the registry shares their IDs.
type Locator struct {
	cfg          *model.GlobalConfig
	currentPaths *config.Paths
	currentStore store.BoardStore
	current      []Board
	loaded       bool
	index        *boardIndex
	strict       bool
}

// NewLocator returns a Locator for the current project at currentPaths,
// whose boards are in current. currentPaths has an empty project root when
// kan runs outside any project.
func NewLocator(cfg *model.GlobalConfig, currentPaths *config.Paths, current store.BoardStore) *Locator {
	return &Locator{cfg: cfg, currentPaths: currentPaths, currentStore: current}
}

// RequireKnownIdentities makes Board fail, rather than name a holder or
// report the board missing, while a visible project's boards can't all be
// identified, since that project could also hold the ID. Doctor needs this
// before it deletes a link as pointing nowhere. Reads don't: one broken
// registered project would otherwise break every link lookup on the machine.
func (l *Locator) RequireKnownIdentities() {
	l.strict = true
}

// Board finds the project holding boardID: the current project's own boards
// first, then the visible registry entries. Hidden entries never count. When
// two visible entries hold the ID, the board is ambiguous.
func (l *Locator) Board(boardID string) (Location, error) {
	current, err := l.currentBoards()
	if err != nil {
		return Location{}, err
	}
	for _, b := range current {
		if b.ID == boardID {
			return Location{Root: l.currentRoot(), DataLocation: l.currentPaths.DataLocation(), Board: b.Name, BoardID: b.ID}, nil
		}
	}

	index := l.boardIndex()
	if l.strict {
		if err := index.unknownIdentity(boardID, l.currentRoot()); err != nil {
			return Location{}, err
		}
	}
	var found []Location
	for _, p := range index.holders(boardID, l.currentRoot()) {
		for _, b := range p.boards {
			if b.ID == boardID {
				found = append(found, Location{Root: p.Path, DataLocation: p.DataLocation, Board: b.Name, BoardID: b.ID})
			}
		}
	}
	switch len(found) {
	case 0:
		return Location{}, kanerr.BoardNotFound(boardID)
	case 1:
		return found[0], nil
	default:
		paths := make([]string, len(found))
		for i, f := range found {
			paths[i] = f.Root
		}
		return Location{}, &kanerr.AmbiguousBoardError{BoardID: boardID, Paths: paths}
	}
}

// FindCard lists the boards in other visible projects that have a card file
// named after cardID. It costs one file lookup per board and parses nothing.
// Boards whose ID the current project holds are skipped: a card there would
// resolve to the current project's copy of the board.
func (l *Locator) FindCard(cardID string) ([]Location, error) {
	if cardID == "" || cardID == "." || cardID == ".." || filepath.Base(cardID) != cardID {
		return nil, nil
	}
	current, err := l.currentBoards()
	if err != nil {
		return nil, err
	}
	ours := map[string]bool{}
	for _, b := range current {
		ours[b.ID] = true
	}

	var found []Location
	for _, e := range l.boardIndex().projects {
		if root := l.currentRoot(); root != "" && SamePath(e.Path, root) {
			continue
		}
		paths := config.NewPaths(e.Path, e.DataLocation)
		for _, b := range e.boards {
			if b.ID == "" || ours[b.ID] {
				continue
			}
			if _, err := os.Stat(paths.CardPath(b.Name, cardID)); err == nil {
				found = append(found, Location{Root: e.Path, DataLocation: e.DataLocation, Board: b.Name, BoardID: b.ID})
			}
		}
	}
	return found, nil
}

func (l *Locator) currentBoards() ([]Board, error) {
	if l.loaded || l.currentRoot() == "" || l.currentStore == nil {
		return l.current, nil
	}
	names, err := l.currentStore.List()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		cfg, err := l.currentStore.Get(name)
		if err != nil {
			return nil, err
		}
		l.current = append(l.current, Board{Name: name, ID: cfg.ID})
	}
	l.loaded = true
	return l.current, nil
}

func (l *Locator) currentRoot() string {
	if l.currentPaths == nil {
		return ""
	}
	return l.currentPaths.ProjectRoot()
}

// boardIndex reads the visible projects' boards on first use, once for
// all of the Locator's lookups.
func (l *Locator) boardIndex() *boardIndex {
	if l.index == nil {
		l.index = newBoardIndex(l.cfg)
	}
	return l.index
}
