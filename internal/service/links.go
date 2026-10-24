package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
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
	State         LinkState
	Ref           string
	BoardID       string
	BoardName     string
	Card          *model.Card
	Column        *model.Column
	Reason        string
	MinKanVersion string
}

type boardLinks struct {
	config   *model.BoardConfig
	err      error
	listing  store.CardListing
	cards    map[string]*model.Card
	failures map[string]error
	parents  *parentGraph
}

// LinkGraph is a command's snapshot of boards, cards, and link lookup failures.
type LinkGraph struct {
	boards map[string]*boardLinks
	byID   map[string][]string
	names  []string
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
	g := &LinkGraph{boards: map[string]*boardLinks{}, byID: map[string][]string{}, names: names}
	for _, name := range names {
		b := &boardLinks{cards: map[string]*model.Card{}, failures: map[string]error{}}
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
		for _, card := range b.listing.Cards {
			b.cards[card.ID] = card
		}
		for _, failure := range b.listing.Failures {
			b.failures[failure.CardID] = failure.Err
		}
		b.parents = newParentGraph(b.listing.Cards)
		g.boards[name] = b
	}
	return g, nil
}

func (g *LinkGraph) Board(boardName string) (*model.BoardConfig, error) {
	b := g.boards[boardName]
	if b == nil {
		return nil, kanerr.BoardNotFound(boardName)
	}
	return b.config, b.err
}

func (g *LinkGraph) Cards(boardName string) ([]*model.Card, error) {
	b := g.boards[boardName]
	if b == nil {
		return nil, kanerr.BoardNotFound(boardName)
	}
	return b.listing.Cards, b.err
}

func (g *LinkGraph) Resolve(boardName, storedRef string) LinkLookup {
	result := LinkLookup{Ref: storedRef, BoardName: boardName}
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
		if len(matches) == 0 {
			for _, name := range g.names {
				b := g.boards[name]
				if b.config == nil && b.err != nil {
					return failedLink(result, b.err)
				}
			}
			result.State = LinkBoardNotHere
			result.Reason = fmt.Sprintf("board %s is not in this project", boardID)
			return result
		}
		boardName = matches[0]
		result.BoardName = boardName
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
		result.Reason = fmt.Sprintf("board %s was not read completely: %d card files could not be read", boardName, len(b.listing.Failures))
		return result
	}
	result.State = LinkAbsent
	result.Reason = fmt.Sprintf("card %s is absent from board %s", cardID, boardName)
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
	for _, name := range g.names {
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
				if target.State == LinkFound && target.BoardName == boardName && target.Card.ID == cardID {
					results = append(results, g.Resolve(name, card.ID))
					break
				}
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].BoardName != results[j].BoardName {
			return results[i].BoardName < results[j].BoardName
		}
		return results[i].Card.ID < results[j].Card.ID
	})
	return results
}
