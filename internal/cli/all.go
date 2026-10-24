package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/util"
)

// allBoard is one board an --all query reads, in the project that holds it.
type allBoard struct {
	Project *App
	Name    string
}

// skippedBoard names a board an --all query couldn't read, and why. Board
// and BoardID are both empty only when kan couldn't tell which boards the
// project has; BoardID alone is empty when the board's config couldn't be
// read.
type skippedBoard struct {
	Path    string `json:"path"`
	Board   string `json:"board"`
	BoardID string `json:"board_id"`
	Reason  string `json:"reason"`
}

// boardsForAll returns the boards an --all query reads: every board of the
// current project, if there is one, and of every visible registered
// project, ordered by project path and then board name. Projects kan can't
// open are skipped whole, because opening one plans the migration of all its
// boards together.
func (a *App) boardsForAll() ([]allBoard, []skippedBoard) {
	type project struct {
		path   string
		open   func() (*App, error)
		boards []registry.Board
		skip   string
	}

	var statuses []registry.Status
	if a.globalCfg != nil {
		statuses = registry.Inspect(a.globalCfg, false)
	}

	// A project is listed once, under one spelling of its path, whichever
	// directory or symlink kan ran from: the registry's entry when it has
	// one (the first by path, if several name the same directory), else the
	// path with symlinks resolved.
	var projects []project
	listed := map[string]bool{}
	if a.RequireKan() == nil {
		current := a
		reported := registry.Canonical(a.ProjectRoot)
		if a.globalCfg != nil {
			for _, e := range registry.Entries(a.globalCfg) {
				if registry.SamePath(e.Path, a.ProjectRoot) {
					reported = e.Path
					break
				}
			}
		}
		if reported != a.ProjectRoot {
			view := *a
			view.ProjectRoot = reported
			current = &view
		}
		listed[registry.Canonical(reported)] = true
		projects = append(projects, project{path: reported, open: func() (*App, error) { return current, nil }})
	}
	for _, status := range statuses {
		key := registry.Canonical(status.Path)
		if listed[key] {
			continue
		}
		listed[key] = true
		p := project{path: status.Path, boards: status.Boards}
		switch status.State {
		case registry.StateMissing:
			p.skip = `project not found; run "kan registry prune" to remove stale entries`
		case registry.StateUnreadable:
			p.skip = status.Error
		default:
			entry := status.Entry
			p.open = func() (*App, error) { return a.openProjectForReading(entry.Path, entry.DataLocation) }
		}
		projects = append(projects, p)
	}
	sort.SliceStable(projects, func(i, j int) bool { return projects[i].path < projects[j].path })

	var boards []allBoard
	var skipped []skippedBoard
	skipProject := func(p project, reason string) {
		if len(p.boards) == 0 {
			skipped = append(skipped, skippedBoard{Path: p.path, Reason: reason})
		}
		for _, b := range p.boards {
			skipped = append(skipped, skippedBoard{Path: p.path, Board: b.Name, BoardID: b.ID, Reason: reason})
		}
	}
	for _, p := range projects {
		if p.skip != "" {
			skipProject(p, p.skip)
			continue
		}
		app, err := p.open()
		if err != nil {
			if inner := errors.Unwrap(err); inner != nil {
				err = inner
			}
			skipProject(p, err.Error())
			continue
		}
		names, err := app.BoardStore.List()
		if err != nil {
			skipProject(p, err.Error())
			continue
		}
		sort.Strings(names)
		for _, name := range names {
			boards = append(boards, allBoard{Project: app, Name: name})
		}
	}
	return boards, skipped
}

// sortSkipped orders skipped boards by project path, the order of the
// results they were left out of.
func sortSkipped(skipped []skippedBoard) {
	sort.SliceStable(skipped, func(i, j int) bool { return skipped[i].Path < skipped[j].Path })
}

// skipBoard records a board that failed to load after its project opened.
func skipBoard(skipped []skippedBoard, b allBoard, boardID string, err error) []skippedBoard {
	return append(skipped, skippedBoard{Path: b.Project.ProjectRoot, Board: b.Name, BoardID: boardID, Reason: err.Error()})
}

// printSkipped notes each skipped board on stderr, so text output says
// which boards it leaves out without changing the exit status.
func printSkipped(skipped []skippedBoard) {
	for _, s := range skipped {
		what := util.PrettyPath(s.Path)
		if s.Board != "" {
			board := s.Board
			if s.BoardID != "" {
				board += " (" + s.BoardID + ")"
			}
			what = "board " + board + " at " + what
		}
		fmt.Fprintf(os.Stderr, "%s skipped %s: %s\n", RenderMuted("Note:"), what, s.Reason)
	}
}

// printBoardHeader starts one board's section of --all text output.
func printBoardHeader(boardName, projectPath string, first bool) {
	if !first {
		fmt.Println()
	}
	fmt.Printf("%s  %s\n", RenderBold(boardName), RenderMuted(util.PrettyPath(projectPath)))
}
