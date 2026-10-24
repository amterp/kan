package registry

import "github.com/amterp/kan/internal/model"

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
		for _, b := range p.boards {
			if b.ID != "" {
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
