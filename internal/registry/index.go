package registry

import (
	"fmt"
	"os"

	"github.com/amterp/kan/internal/model"
)

// readBoards is ReadBoards, replaced by tests that count project reads.
var readBoards = ReadBoards

// boardIndex is the visible projects' boards, read once. A query across
// projects asks it which projects hold a board ID instead of reading every
// project per question, which would make the query quadratic in the number
// of registered projects.
type boardIndex struct {
	projects []indexedProject
	byID     map[string][]int
}

type indexedProject struct {
	Entry
	boards []Board
	err    error
}

// newBoardIndex reads the boards of every visible entry. Entries naming the
// same directory through a symlink are one project: the first by path is
// kept, so its spelling is the one reported.
func newBoardIndex(cfg *model.GlobalConfig) *boardIndex {
	x := &boardIndex{byID: map[string][]int{}}
	if cfg == nil {
		return x
	}
	seen := map[string]bool{}
	for _, e := range Visible(cfg) {
		key := Canonical(e.Path)
		if seen[key] {
			continue
		}
		seen[key] = true
		p := indexedProject{Entry: e}
		p.boards, p.err = readBoards(e.Path, e.DataLocation)
		x.projects = append(x.projects, p)
		indexed := map[string]bool{}
		for _, b := range p.boards {
			if b.ID != "" && !indexed[b.ID] {
				indexed[b.ID] = true
				x.byID[b.ID] = append(x.byID[b.ID], len(x.projects)-1)
			}
		}
	}
	return x
}

// holders returns the projects other than exclude that hold boardID, by the
// index's spelling of their paths, sorted.
func (x *boardIndex) holders(boardID, exclude string) []indexedProject {
	var found []indexedProject
	for _, i := range x.byID[boardID] {
		if p := x.projects[i]; exclude == "" || !SamePath(p.Path, exclude) {
			found = append(found, p)
		}
	}
	return found
}

// unknownIdentity reports a project other than exclude whose boards can't
// all be identified: one kan couldn't read, or a board with no ID. Either
// could hold boardID, so no lookup can show that a board is unique, or
// absent. A project whose directory is gone holds nothing and is skipped.
func (x *boardIndex) unknownIdentity(boardID, exclude string) error {
	for _, p := range x.projects {
		if exclude != "" && SamePath(p.Path, exclude) {
			continue
		}
		if p.err != nil {
			if os.IsNotExist(p.err) {
				continue
			}
			return fmt.Errorf("cannot establish a unique holder of board %s: reading %s: %w", boardID, p.Path, p.err)
		}
		for _, b := range p.boards {
			if b.ID == "" {
				return fmt.Errorf("cannot establish a unique holder of board %s: board %s at %s has no ID", boardID, b.Name, p.Path)
			}
		}
	}
	return nil
}

func paths(projects []indexedProject) []string {
	var out []string
	for _, p := range projects {
		out = append(out, p.Path)
	}
	return out
}

// boardsOf returns e's boards, from the index when it holds e.
func (x *boardIndex) boardsOf(e Entry) ([]Board, error) {
	for _, p := range x.projects {
		if p.Path == e.Path {
			return p.boards, p.err
		}
	}
	return readBoards(e.Path, e.DataLocation)
}
