package service

import (
	"fmt"
	"slices"
	"strings"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/resolver"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
)

type LinkService struct {
	cards  store.CardStore
	boards store.BoardStore
}

func NewLinkService(cards store.CardStore, boards store.BoardStore) *LinkService {
	return &LinkService{cards: cards, boards: boards}
}

func (s *LinkService) Link(boardName, cardRef string, blockedBy, related []string) (*model.Card, error) {
	g, err := LoadLinkGraph(s.boards, s.cards)
	if err != nil {
		return nil, err
	}
	cards, err := g.Cards(boardName)
	if err != nil {
		return nil, err
	}
	original, err := resolver.ResolveInCards(cards, cardRef)
	if err != nil {
		return nil, err
	}
	card := *original
	if err := s.Prepare(g, boardName, &card, blockedBy, related); err != nil {
		return nil, err
	}
	if slices.Equal(original.BlockedBy, card.BlockedBy) && slices.Equal(original.Related, card.Related) {
		return original, nil
	}
	card.UpdatedAtMillis = util.NowMillis()
	if err := s.cards.Update(boardName, &card); err != nil {
		return nil, err
	}
	return &card, nil
}

// Prepare validates all requested links before changing a candidate card.
func (s *LinkService) Prepare(g *LinkGraph, boardName string, card *model.Card, blockedBy, related []string) error {
	cards, err := g.Cards(boardName)
	if err != nil {
		return err
	}
	resolve := func(refs []string, existing []string) ([]string, error) {
		result := slices.Clone(existing)
		for _, ref := range refs {
			if strings.Contains(ref, ":") {
				return nil, kanerr.InvalidField("link", "cross-board links are not supported yet: "+ref)
			}
			target, err := resolver.ResolveInCards(cards, ref)
			if err != nil {
				return nil, fmt.Errorf("cannot link to %s: %w", ref, err)
			}
			if !slices.Contains(result, target.ID) {
				result = append(result, target.ID)
			}
		}
		return result, nil
	}
	newBlocked, err := resolve(blockedBy, card.BlockedBy)
	if err != nil {
		return err
	}
	newRelated, err := resolve(related, card.Related)
	if err != nil {
		return err
	}
	proposed := *card
	proposed.BlockedBy = newBlocked
	if len(blockedBy) > 0 {
		if err := g.validateBlockers(boardName, &proposed); err != nil {
			return err
		}
	}
	card.BlockedBy, card.Related = newBlocked, newRelated
	return nil
}

func (g *LinkGraph) validateBlockers(boardName string, card *model.Card) error {
	b := g.boards[boardName]
	ancestors := map[string]bool{}
	for parent := b.parents.resolve(card.Parent); parent != nil && !ancestors[parent.ID]; parent = b.parents.resolve(parent.Parent) {
		ancestors[parent.ID] = true
	}
	for _, ref := range card.BlockedBy {
		blocker := g.Resolve(boardName, ref)
		if blocker.State == LinkFound && blocker.BoardName == boardName && ancestors[blocker.Card.ID] {
			return kanerr.InvalidField("blocked_by", fmt.Sprintf("card %s cannot be blocked by its ancestor %s", card.ID, blocker.Card.ID))
		}
	}
	seen := map[string]bool{}
	var walk func(string, string, []string) error
	walk = func(fromBoard, ref string, path []string) error {
		target := g.Resolve(fromBoard, ref)
		isCandidate := fromBoard == boardName && ref == card.ID
		if boardID, id, qualified := strings.Cut(ref, ":"); qualified && id == card.ID && b.config.ID == boardID && len(g.byID[boardID]) == 1 {
			isCandidate = true
		}
		if isCandidate || (target.State == LinkFound && target.BoardName == boardName && target.Card.ID == card.ID) {
			return kanerr.InvalidField("blocked_by", "blocker cycle: "+strings.Join(append(path, card.ID), " -> "))
		}
		if target.State != LinkFound {
			return nil
		}
		key := target.BoardName + "\x00" + target.Card.ID
		if seen[key] {
			return nil
		}
		seen[key] = true
		path = append(path, target.Card.ID)
		for _, next := range target.Card.BlockedBy {
			if err := walk(target.BoardName, next, path); err != nil {
				return err
			}
		}
		return nil
	}
	for _, ref := range card.BlockedBy {
		if err := walk(boardName, ref, []string{card.ID}); err != nil {
			return err
		}
	}
	return nil
}

func (s *LinkService) Unlink(boardName, cardRef string, refs []string) (*model.Card, error) {
	g, err := LoadLinkGraph(s.boards, s.cards)
	if err != nil {
		return nil, err
	}
	cards, err := g.Cards(boardName)
	if err != nil {
		return nil, err
	}
	original, err := resolver.ResolveInCards(cards, cardRef)
	if err != nil {
		return nil, err
	}
	remove := map[string]bool{}
	for _, ref := range refs {
		if slices.Contains(original.BlockedBy, ref) || slices.Contains(original.Related, ref) {
			remove[ref] = true
			continue
		}
		if strings.Contains(ref, ":") {
			continue
		}
		target, err := resolver.ResolveInCards(cards, ref)
		if err != nil {
			if kanerr.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		remove[target.ID] = true
	}
	filter := func(refs []string) []string {
		var result []string
		for _, ref := range refs {
			if !remove[ref] {
				result = append(result, ref)
			}
		}
		return result
	}
	card := *original
	card.BlockedBy, card.Related = filter(card.BlockedBy), filter(card.Related)
	if slices.Equal(card.BlockedBy, original.BlockedBy) && slices.Equal(card.Related, original.Related) {
		return original, nil
	}
	card.UpdatedAtMillis = util.NowMillis()
	if err := s.cards.Update(boardName, &card); err != nil {
		return nil, err
	}
	return &card, nil
}
