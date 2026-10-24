package service

import (
	"fmt"
	"sort"
	"strings"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
)

type parentGraph struct {
	cards   map[string]*model.Card
	aliases map[string]*model.Card
}

func newParentGraph(cards []*model.Card) *parentGraph {
	g := &parentGraph{
		cards:   make(map[string]*model.Card, len(cards)),
		aliases: make(map[string]*model.Card, len(cards)),
	}
	for _, card := range cards {
		g.cards[card.ID] = card
		if card.Alias != "" {
			g.aliases[card.Alias] = card
		}
	}
	return g
}

func (g *parentGraph) resolve(ref string) *model.Card {
	if card := g.cards[ref]; card != nil {
		return card
	}
	return g.aliases[ref]
}

func (g *parentGraph) validate(cardID, parentRef, alias string) (string, error) {
	if parentRef == "" {
		return "", nil
	}
	parent := g.resolve(parentRef)
	if parent == nil && parentRef == alias {
		return "", kanerr.InvalidField("parent", fmt.Sprintf("card %s cannot be its own parent", cardID))
	}
	if parent == nil {
		return "", kanerr.InvalidField("parent", fmt.Sprintf("parent card not found: %s", parentRef))
	}
	if parent.ID == cardID {
		return "", kanerr.InvalidField("parent", fmt.Sprintf("card %s cannot be its own parent", cardID))
	}

	path := []string{}
	seen := map[string]bool{}
	if cardID != "" {
		path = append(path, cardID)
		seen[cardID] = true
	}
	// Legacy parent aliases can start or stop resolving when a card's alias changes.
	pending := &model.Card{ID: cardID}
	resolveAfterWrite := func(ref string) *model.Card {
		if ref == "" {
			return nil
		}
		if ref == cardID {
			return pending
		}
		if card := g.cards[ref]; card != nil {
			return card
		}
		if ref == alias {
			return pending
		}
		card := g.aliases[ref]
		if card != nil && card.ID == cardID {
			return nil
		}
		return card
	}
	for current := parent; current != nil; current = resolveAfterWrite(current.Parent) {
		path = append(path, current.ID)
		if seen[current.ID] {
			return "", kanerr.InvalidField("parent", "parent cycle: "+strings.Join(path, " -> "))
		}
		seen[current.ID] = true
	}
	return parent.ID, nil
}

func (g *parentGraph) cycles() [][]string {
	ids := make([]string, 0, len(g.cards))
	for id := range g.cards {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	finished := make(map[string]bool, len(ids))
	var cycles [][]string
	for _, id := range ids {
		path := []string{}
		indices := map[string]int{}
		for current := g.cards[id]; current != nil && !finished[current.ID]; current = g.resolve(current.Parent) {
			if index, ok := indices[current.ID]; ok {
				cycle := append([]string(nil), path[index:]...)
				cycle = append(cycle, current.ID)
				cycles = append(cycles, cycle)
				break
			}
			indices[current.ID] = len(path)
			path = append(path, current.ID)
		}
		for _, visited := range path {
			finished[visited] = true
		}
	}
	return cycles
}
