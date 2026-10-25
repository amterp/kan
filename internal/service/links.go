package service

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

type LinkState string

const (
	LinkFound             LinkState = "found"
	LinkAbsent            LinkState = "absent"
	LinkBoardNotHere      LinkState = "board_not_here"
	LinkAmbiguousBoard    LinkState = "ambiguous_board"
	LinkUnsupportedSchema LinkState = "unsupported_schema"
	LinkUnreadable        LinkState = "unreadable"
)

type LinkLookup struct {
	State     LinkState
	Ref       string
	BoardID   string
	BoardName string
	// Project is the root of the project holding the board, when that is a
	// project other than the one the graph was loaded from.
	Project       string
	Card          *model.Card
	Column        *model.Column
	Reason        string
	MinKanVersion string
	// LinkedName and LinkedRemote describe a board that isn't on this
	// machine, from the current project's [linked_boards], when recorded.
	LinkedName   string
	LinkedRemote string

	// board is the graph's key for the board: its name for the graph's own
	// boards, otherKey(id) for another project's.
	board string
}

// Elsewhere reaches boards outside the project a LinkGraph was loaded from.
// Without it, a link to such a board resolves as board_not_here.
type Elsewhere struct {
	Locator *registry.Locator
	// Open opens another project for reading.
	Open func(root, dataLocation string) (store.BoardStore, store.CardStore, error)
	// Projects are the projects reverse links are gathered from: the
	// visible registry entries and the current project.
	Projects []registry.Entry
	// LinkedBoards is the current project's [linked_boards], used only to
	// describe a board that isn't on this machine.
	LinkedBoards map[string]model.LinkedBoard
	// Remote returns a URL the repo at root can be fetched from, or "".
	Remote func(root string) string
}

// SkippedProject is a project LoadOtherProjects couldn't read.
type SkippedProject struct {
	Path   string
	Reason string
}

type boardLinks struct {
	name         string
	project      string
	dataLocation string
	config       *model.BoardConfig
	err          error
	listing      store.CardListing
	cards        map[string]*model.Card
	failures     map[string]error
	parents      *parentGraph
}

// LinkGraph is a command's snapshot of boards, cards, and link lookup failures.
// It holds the boards of the project it was loaded from and, with
// Elsewhere, the boards of other projects that its links reach, each read
// once per command.
type LinkGraph struct {
	boards map[string]*boardLinks
	byID   map[string][]string
	names  []string

	elsewhere *Elsewhere
	projects  map[string]*openedProject
	other     []string
	unplaced  map[string]error
}

type openedProject struct {
	boards store.BoardStore
	cards  store.CardStore
	err    error
}

// otherKey is the graph's key for another project's board. It can't collide
// with a board name.
func otherKey(boardID string) string {
	return "\x00" + boardID
}

// boardLabel names the board a lookup found: its name in this project, or
// project/board in another, as [linked_boards] records it.
func (g *LinkGraph) boardLabel(found LinkLookup) string {
	b := g.boards[found.board]
	if b == nil || b.project == "" {
		return found.BoardName
	}
	return registry.ProjectName(b.project, b.dataLocation) + "/" + found.BoardName
}

// SetElsewhere lets the graph follow links to boards in other projects.
func (g *LinkGraph) SetElsewhere(e *Elsewhere) {
	g.elsewhere = e
}

func LoadLinkGraph(boards store.BoardStore, cards store.CardStore) (*LinkGraph, error) {
	return loadLinkGraph(boards, cards, "", nil, store.CardListing{})
}

func loadLinkGraph(boards store.BoardStore, cards store.CardStore, seedName string, seedConfig *model.BoardConfig, seedListing store.CardListing) (*LinkGraph, error) {
	names, err := boards.List()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	g := &LinkGraph{
		boards:   map[string]*boardLinks{},
		byID:     map[string][]string{},
		names:    names,
		projects: map[string]*openedProject{},
		unplaced: map[string]error{},
	}
	for _, name := range names {
		b := &boardLinks{name: name}
		if name == seedName {
			b.config, b.listing = seedConfig, seedListing
		} else {
			b.config, b.err = boards.Get(name)
			if b.err == nil {
				b.listing, b.err = cards.ListWithDiagnostics(name)
			}
		}
		if b.config != nil && b.config.ID != "" {
			g.byID[b.config.ID] = append(g.byID[b.config.ID], name)
		}
		b.index()
		g.boards[name] = b
	}
	return g, nil
}

func (b *boardLinks) index() {
	b.cards = map[string]*model.Card{}
	b.failures = map[string]error{}
	for _, card := range b.listing.Cards {
		b.cards[card.ID] = card
	}
	for _, failure := range b.listing.Failures {
		b.failures[failure.CardID] = failure.Err
	}
	b.parents = newParentGraph(b.listing.Cards)
}

func (g *LinkGraph) openProject(root, dataLocation string) *openedProject {
	if p := g.projects[root]; p != nil {
		return p
	}
	p := &openedProject{}
	p.boards, p.cards, p.err = g.elsewhere.Open(root, dataLocation)
	g.projects[root] = p
	return p
}

// addOtherBoard reads a board of another project into the graph.
func (g *LinkGraph) addOtherBoard(loc registry.Location, p *openedProject) string {
	key := otherKey(loc.BoardID)
	b := &boardLinks{name: loc.Board, project: loc.Root, dataLocation: loc.DataLocation}
	b.config, b.err = p.boards.Get(loc.Board)
	if b.err == nil {
		b.listing, b.err = p.cards.ListWithDiagnostics(loc.Board)
	}
	b.index()
	g.boards[key] = b
	g.other = append(g.other, key)
	return key
}

// otherBoard finds and reads the board with boardID in another project. On
// failure it fills in result's state and returns false.
func (g *LinkGraph) otherBoard(boardID string, result *LinkLookup) (string, bool) {
	key := otherKey(boardID)
	if g.boards[key] != nil {
		return key, true
	}
	err := g.unplaced[boardID]
	var loc registry.Location
	if err == nil && g.elsewhere != nil {
		loc, err = g.elsewhere.Locator.Board(boardID)
		if err != nil {
			g.unplaced[boardID] = err
		}
	}

	var ambiguous *kanerr.AmbiguousBoardError
	switch {
	case g.elsewhere == nil || kanerr.IsNotFound(err):
		result.State = LinkBoardNotHere
		result.Reason = fmt.Sprintf("board %s isn't on this machine", boardID)
		if g.elsewhere != nil {
			linked := g.elsewhere.LinkedBoards[boardID]
			result.LinkedName, result.LinkedRemote = linked.Name, linked.Remote
		}
		return "", false
	case errors.As(err, &ambiguous):
		result.State, result.Reason = LinkAmbiguousBoard, err.Error()
		return "", false
	case err != nil:
		*result = failedLink(*result, err)
		return "", false
	}

	p := g.openProject(loc.Root, loc.DataLocation)
	if p.err != nil {
		result.Project, result.BoardName = loc.Root, loc.Board
		*result = failedLink(*result, p.err)
		return "", false
	}
	return g.addOtherBoard(loc, p), true
}

// boardForRef finds the board a link argument names: a board name in the
// graph's own project, a board ID there, or, with Elsewhere, a board ID in
// another registered project.
func (g *LinkGraph) boardForRef(ref string) (string, error) {
	if slices.Contains(g.names, ref) {
		return ref, nil
	}
	switch matches := g.byID[ref]; len(matches) {
	case 1:
		return matches[0], nil
	case 0:
	default:
		return "", fmt.Errorf("board ID %s belongs to multiple boards: %s", ref, strings.Join(matches, ", "))
	}
	var result LinkLookup
	key, ok := g.otherBoard(ref, &result)
	switch {
	case ok:
		return key, nil
	case result.State == LinkBoardNotHere:
		return "", kanerr.BoardNotFound(ref)
	default:
		return "", errors.New(result.Reason)
	}
}

// describe sets result's board fields for the board at key.
func (g *LinkGraph) describe(result *LinkLookup, key string) {
	result.board, result.BoardName = key, key
	if b := g.boards[key]; b != nil {
		result.BoardName, result.Project = b.name, b.project
	}
}

// NotOnThisMachine explains a link from waitingID to a board that isn't on
// this machine, naming the board and where to get it when that's recorded.
func NotOnThisMachine(waitingID string, l LinkLookup) string {
	board := l.BoardID
	if l.LinkedName != "" {
		board = fmt.Sprintf("%s (%s)", l.LinkedName, l.BoardID)
	}
	msg := fmt.Sprintf("%s waits on %s, which isn't on this machine.", waitingID, board)
	if l.LinkedRemote != "" {
		msg += "\nOne place to get it: " + l.LinkedRemote
	}
	return msg
}

// LoadOtherProjects reads every board of Elsewhere's projects, so reverse
// links include cards in other repos. Boards the graph's own project holds
// are skipped (its own boards win over clones), and so are boards whose ID
// two visible projects hold. A project that can't be read is returned, not
// an error.
func (g *LinkGraph) LoadOtherProjects() []SkippedProject {
	if g.elsewhere == nil {
		return nil
	}
	var skipped []SkippedProject
	reported := map[string]bool{}
	for _, e := range g.elsewhere.Projects {
		p := g.openProject(e.Path, e.DataLocation)
		if p.err != nil {
			skipped = append(skipped, SkippedProject{Path: e.Path, Reason: p.err.Error()})
			continue
		}
		names, err := p.boards.List()
		if err != nil {
			skipped = append(skipped, SkippedProject{Path: e.Path, Reason: err.Error()})
			continue
		}
		for _, name := range names {
			cfg, err := p.boards.Get(name)
			if err != nil {
				skipped = append(skipped, SkippedProject{Path: e.Path, Reason: fmt.Sprintf("board %s: %v", name, err)})
				continue
			}
			if cfg.ID == "" || len(g.byID[cfg.ID]) > 0 || g.boards[otherKey(cfg.ID)] != nil {
				continue
			}
			loc, err := g.elsewhere.Locator.Board(cfg.ID)
			if err != nil {
				if !reported[cfg.ID] {
					reported[cfg.ID] = true
					skipped = append(skipped, SkippedProject{Path: e.Path, Reason: err.Error()})
				}
				continue
			}
			if registry.SamePath(loc.Root, e.Path) {
				g.addOtherBoard(loc, p)
			}
		}
	}
	return skipped
}

// Board returns a board of the graph's own project.
func (g *LinkGraph) Board(boardName string) (*model.BoardConfig, error) {
	b := g.boards[boardName]
	if b == nil {
		return nil, kanerr.BoardNotFound(boardName)
	}
	return b.config, b.err
}

func (g *LinkGraph) LookupBoard(result LinkLookup) (*model.BoardConfig, error) {
	return g.Board(result.board)
}

func (g *LinkGraph) ResolveFrom(source LinkLookup, ref string) LinkLookup {
	return g.Resolve(source.board, ref)
}

func (g *LinkGraph) BlocksFor(target LinkLookup) []LinkLookup {
	return g.Blocks(target.board, target.Card.ID)
}

func (g *LinkGraph) Cards(boardName string) ([]*model.Card, error) {
	b := g.boards[boardName]
	if b == nil {
		return nil, kanerr.BoardNotFound(boardName)
	}
	return b.listing.Cards, b.err
}

// Failures returns the board's card files that could not be read.
func (g *LinkGraph) Failures(boardName string) []store.CardReadFailure {
	if b := g.boards[boardName]; b != nil {
		return b.listing.Failures
	}
	return nil
}

// Resolve looks up a stored ref found on the board at boardName (a board of
// the graph's own project, or a key from an earlier lookup). A qualified
// ref's board is looked up in the graph's own project first, then, with
// Elsewhere, among the registered projects. Stored refs match exact IDs
// only.
func (g *LinkGraph) Resolve(boardName, storedRef string) LinkLookup {
	result := LinkLookup{Ref: storedRef}
	g.describe(&result, boardName)
	cardID := storedRef
	if boardID, ref, qualified := strings.Cut(storedRef, ":"); qualified {
		result.BoardID = boardID
		cardID = ref
		matches := g.byID[boardID]
		if len(matches) > 1 {
			result.State = LinkAmbiguousBoard
			result.Reason = fmt.Sprintf("board ID %s belongs to multiple boards: %s", boardID, strings.Join(matches, ", "))
			return result
		}
		for _, name := range g.names {
			b := g.boards[name]
			if b.config != nil && b.config.ID != "" {
				continue
			}
			err := b.err
			if err == nil {
				err = fmt.Errorf("board %s has no readable ID", name)
			}
			return failedLink(result, err)
		}
		if len(matches) == 1 {
			boardName = matches[0]
		} else {
			key, ok := g.otherBoard(boardID, &result)
			if !ok {
				return result
			}
			boardName = key
		}
		g.describe(&result, boardName)
	}
	b := g.boards[boardName]
	if b == nil {
		result.State = LinkBoardNotHere
		result.Reason = fmt.Sprintf("board %s is not in this project", boardName)
		return result
	}
	if b.config != nil {
		result.BoardID = b.config.ID
	}
	if b.err != nil {
		return failedLink(result, b.err)
	}
	if card := b.cards[cardID]; card != nil {
		result.State, result.Card = LinkFound, card
		result.Column = b.config.GetColumn(card.Column)
		return result
	}
	if err := b.failures[cardID]; err != nil {
		return failedLink(result, err)
	}
	if len(b.listing.Failures) > 0 {
		result.State = LinkUnreadable
		result.Reason = fmt.Sprintf("board %s was not read completely: %d card files could not be read", result.BoardName, len(b.listing.Failures))
		return result
	}
	result.State = LinkAbsent
	result.Reason = fmt.Sprintf("card %s is absent from board %s", cardID, result.BoardName)
	return result
}

func failedLink(result LinkLookup, err error) LinkLookup {
	result.State, result.Reason = LinkUnreadable, err.Error()
	var schema *version.SchemaVersionError
	if errors.As(err, &schema) && schema.MinRequired != "" {
		result.State, result.MinKanVersion = LinkUnsupportedSchema, schema.MinRequired
	}
	return result
}

func (g *LinkGraph) Blockers(boardName string, card *model.Card) []LinkLookup {
	results := make([]LinkLookup, 0, len(card.BlockedBy))
	for _, ref := range card.BlockedBy {
		results = append(results, g.Resolve(boardName, ref))
	}
	return results
}

func (g *LinkGraph) Blocks(boardName, cardID string) []LinkLookup {
	return g.reverse(boardName, cardID, false)
}

func (g *LinkGraph) RelatedTo(boardName, cardID string) []LinkLookup {
	return g.reverse(boardName, cardID, true)
}

func (g *LinkGraph) reverse(boardName, cardID string, related bool) []LinkLookup {
	var results []LinkLookup
	for _, name := range append(slices.Clone(g.names), g.other...) {
		b := g.boards[name]
		if b.err != nil {
			continue
		}
		for _, card := range b.listing.Cards {
			refs := card.BlockedBy
			if related {
				refs = card.Related
			}
			for _, ref := range refs {
				target := g.Resolve(name, ref)
				if target.State == LinkFound && target.board == boardName && target.Card.ID == cardID {
					results = append(results, g.Resolve(name, card.ID))
					break
				}
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Project != results[j].Project {
			return results[i].Project < results[j].Project
		}
		if results[i].BoardName != results[j].BoardName {
			return results[i].BoardName < results[j].BoardName
		}
		return results[i].Card.ID < results[j].Card.ID
	})
	return results
}
