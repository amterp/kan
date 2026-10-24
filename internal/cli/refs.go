package cli

import (
	"errors"
	"fmt"
	"strings"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/resolver"
)

// access says whether a command only reads what it resolves or changes it.
// A command may change a card or board only in its own project: the one kan
// runs in, or the -g board's project.
type access int

const (
	forRead access = iota
	forWrite
)

// CardResolution holds the result of resolving a card along with its board.
type CardResolution struct {
	Card      *model.Card
	BoardName string
	// Project holds the card: the App itself, or another registered project
	// opened read-only.
	Project *App
	// CrossBoard is true when the card was found via cross-board search
	// (no explicit board flag, no single board, no default). Callers can use
	// this to decide whether to print which board the card was found in.
	CrossBoard bool
	// MultipleBoards is true when the project has more than one board.
	// Used by callers to decide whether to show board context in output.
	MultipleBoards bool
}

// otherProjectPath is the project's path when it is a registered project
// other than the current one, for JSON output; "" otherwise.
func otherProjectPath(project *App) string {
	if !project.OtherProject {
		return ""
	}
	return project.ProjectRoot
}

func (a *App) locator() *registry.Locator {
	return registry.NewLocator(a.globalCfg, a.Paths, a.BoardStore)
}

// ResolveBoard resolves a board ref: a board name in the App's project, or a
// board ID. A board ID not in the App's project is looked up among the
// visible registered projects; a read then gets that project opened
// read-only, and a write is refused. An empty ref infers the board in the
// App's project as before. With -g, refs stay inside the global board's
// project.
func (a *App) ResolveBoard(ref string, interactive bool, acc access) (*App, string, error) {
	hasProject := a.RequireKan()
	if ref == "" || a.UsingGlobalBoard {
		if hasProject != nil {
			return nil, "", hasProject
		}
		name, err := a.BoardResolver.Resolve(ref, interactive)
		return a, name, err
	}

	if hasProject == nil {
		name, err := a.BoardResolver.Resolve(ref, interactive)
		if !kanerr.IsNotFound(err) {
			return a, name, err
		}
	}

	loc, err := a.locator().Board(ref)
	if err != nil {
		if kanerr.IsNotFound(err) && hasProject != nil {
			return nil, "", hasProject
		}
		return nil, "", err
	}
	if acc == forWrite {
		return nil, "", &kanerr.OtherProjectError{BoardID: loc.BoardID, Path: loc.Root}
	}
	project, err := a.openProjectForReading(loc.Root, loc.DataLocation)
	if err != nil {
		return nil, "", err
	}
	return project, loc.Board, nil
}

// ResolveCardWithBoard resolves the card a command argument names, and its
// board:
//   - board:card looks only on that board, which may be in another
//     registered project.
//   - With -b or -g, it looks only on that board.
//   - A bare argument is matched as an exact card ID on the App's project's
//     boards, then as a card file name across the other visible registered
//     projects, then as an alias or fuzzy match in the App's project only,
//     since aliases are unique only within a board.
//
// A card outside the App's project can be read; changing it is refused
// before anything touches that project.
func (a *App) ResolveCardWithBoard(explicitBoard, ref string, interactive bool, acc access) (*CardResolution, error) {
	if boardRef, cardRef, ok := strings.Cut(ref, ":"); ok {
		if explicitBoard != "" || a.UsingGlobalBoard {
			return nil, errors.New("use either -b/-g or a board:card reference, not both")
		}
		if boardRef == "" || cardRef == "" {
			return nil, fmt.Errorf("invalid card reference %q: expected board:card", ref)
		}
		project, boardName, err := a.ResolveBoard(boardRef, false, forRead)
		if err != nil {
			return nil, err
		}
		return a.resolveOnBoard(project, boardName, cardRef, acc)
	}

	// -g pins the designated board; card lookups there never search wider.
	if explicitBoard != "" || a.UsingGlobalBoard {
		project, boardName, err := a.ResolveBoard(explicitBoard, false, forRead)
		if err != nil {
			return nil, err
		}
		return a.resolveOnBoard(project, boardName, ref, acc)
	}

	hasProject := a.RequireKan()
	if hasProject == nil {
		if result, err := a.resolveExactID(ref); result != nil || err != nil {
			return result, err
		}
	}
	if result, err := a.resolveInRegistry(ref, acc); result != nil || err != nil {
		return result, err
	}
	if hasProject != nil {
		return nil, hasProject
	}
	return a.resolveInProject(ref, interactive)
}

// resolveOnBoard resolves cardRef (ID, alias, fuzzy) on one board of project.
func (a *App) resolveOnBoard(project *App, boardName, cardRef string, acc access) (*CardResolution, error) {
	card, err := project.CardResolver.Resolve(boardName, cardRef)
	if err != nil {
		return nil, err
	}
	return a.checkAccess(&CardResolution{
		Card:           card,
		BoardName:      boardName,
		Project:        project,
		MultipleBoards: project.hasMultipleBoards(),
	}, acc)
}

// checkAccess refuses a write to a card outside the App's project.
func (a *App) checkAccess(result *CardResolution, acc access) (*CardResolution, error) {
	if acc == forRead || result.Project == a {
		return result, nil
	}
	boardID := ""
	if cfg, err := result.Project.BoardStore.Get(result.BoardName); err == nil {
		boardID = cfg.ID
	}
	return nil, &kanerr.OtherProjectError{CardID: result.Card.ID, BoardID: boardID, Path: result.Project.ProjectRoot}
}

// resolveExactID finds a card whose ID is ref on exactly one of the App's
// boards. It returns nil without an error when no board, or more than one,
// has it; the full search then reports or prompts.
func (a *App) resolveExactID(ref string) (*CardResolution, error) {
	boards, err := a.BoardStore.List()
	if err != nil {
		return nil, err
	}
	var result *CardResolution
	for _, board := range boards {
		card, err := a.CardStore.Get(board, ref)
		if err != nil {
			continue
		}
		if result != nil {
			return nil, nil
		}
		result = &CardResolution{Card: card, BoardName: board, Project: a, MultipleBoards: len(boards) > 1}
	}
	if result != nil && result.MultipleBoards {
		inferred, _ := a.BoardResolver.Resolve("", false)
		result.CrossBoard = result.BoardName != inferred
	}
	return result, nil
}

// resolveInRegistry finds a card whose ID is ref in another visible
// registered project. A card ID on two boards is an error listing each, and
// a board held by two visible projects is ambiguous. Returns nil without an
// error when no other project has it.
func (a *App) resolveInRegistry(ref string, acc access) (*CardResolution, error) {
	locator := a.locator()
	found, err := locator.FindCard(ref)
	if err != nil || len(found) == 0 {
		return nil, err
	}

	boardIDs := map[string]bool{}
	var refs []string
	for _, loc := range found {
		boardIDs[loc.BoardID] = true
		refs = append(refs, fmt.Sprintf("%s:%s (%s)", loc.BoardID, ref, loc.Root))
	}
	if len(boardIDs) > 1 {
		return nil, fmt.Errorf("card %q is on more than one board: %s; use board:card",
			ref, strings.Join(refs, ", "))
	}

	loc, err := locator.Board(found[0].BoardID)
	if err != nil {
		return nil, err
	}
	if acc == forWrite {
		return nil, &kanerr.OtherProjectError{CardID: ref, BoardID: loc.BoardID, Path: loc.Root}
	}
	project, err := a.openProjectForReading(loc.Root, loc.DataLocation)
	if err != nil {
		return nil, err
	}
	card, err := project.CardStore.Get(loc.Board, ref)
	if err != nil {
		return nil, err
	}
	return &CardResolution{Card: card, BoardName: loc.Board, Project: project, MultipleBoards: project.hasMultipleBoards()}, nil
}

// resolveInProject resolves a bare ref by ID, alias or fuzzy match in the
// App's project: on the inferred board first, then across its boards. When
// no board is specified and there are fewer than MaxBoardsForCrossSearch
// boards, it searches across all boards automatically.
func (a *App) resolveInProject(idOrAlias string, interactive bool) (*CardResolution, error) {
	// 1. Try non-interactive inference (single board, default_board).
	boardName, err := a.BoardResolver.Resolve("", false)
	if err == nil {
		card, cardErr := a.CardResolver.Resolve(boardName, idOrAlias)
		if cardErr == nil {
			multipleBoards := a.hasMultipleBoards()
			return &CardResolution{Card: card, BoardName: boardName, Project: a, MultipleBoards: multipleBoards}, nil
		}
		// Card not found on the inferred board. If there are multiple boards,
		// fall through to cross-board search rather than failing - the inferred
		// board is a convenience default, not an explicit user choice.
		if !kanerr.IsNotFound(cardErr) {
			return nil, cardErr
		}
	}

	// 2. Inference failed or card wasn't on the inferred board - try cross-board search
	boards, listErr := a.BoardStore.List()
	if listErr != nil {
		return nil, listErr
	}
	multipleBoards := len(boards) > 1

	if len(boards) >= resolver.MaxBoardsForCrossSearch {
		// Too many boards for cross-board search, fall back to picker
		if !interactive {
			return nil, fmt.Errorf("multiple boards exist; specify with -b or set default_board in config")
		}
		boardName, err = a.Prompter.Select("Select board", boards)
		if err != nil {
			return nil, err
		}
		card, cardErr := a.CardResolver.Resolve(boardName, idOrAlias)
		if cardErr != nil {
			return nil, cardErr
		}
		return &CardResolution{Card: card, BoardName: boardName, Project: a, MultipleBoards: multipleBoards}, nil
	}

	// Cross-board search
	matches, err := a.CardResolver.ResolveAcrossBoards(boards, idOrAlias)
	if err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, kanerr.CardNotFound(idOrAlias)
	case 1:
		return &CardResolution{
			Card:           matches[0].Card,
			BoardName:      matches[0].BoardName,
			Project:        a,
			CrossBoard:     true,
			MultipleBoards: multipleBoards,
		}, nil
	default:
		if !interactive {
			boardNames := make([]string, len(matches))
			for i, m := range matches {
				boardNames[i] = m.BoardName
			}
			return nil, fmt.Errorf("card %q found in multiple boards (%s); specify with -b",
				idOrAlias, strings.Join(boardNames, ", "))
		}
		matchBoards := make([]string, len(matches))
		for i, m := range matches {
			matchBoards[i] = m.BoardName
		}
		selected, selectErr := a.Prompter.Select("Card found in multiple boards - select one", matchBoards)
		if selectErr != nil {
			return nil, selectErr
		}
		for _, m := range matches {
			if m.BoardName == selected {
				return &CardResolution{
					Card:           m.Card,
					BoardName:      m.BoardName,
					Project:        a,
					CrossBoard:     true,
					MultipleBoards: multipleBoards,
				}, nil
			}
		}
		return nil, fmt.Errorf("internal error: selected board not in matches")
	}
}

// hasMultipleBoards returns true if more than one board exists.
func (a *App) hasMultipleBoards() bool {
	boards, err := a.BoardStore.List()
	if err != nil {
		return false
	}
	return len(boards) > 1
}
