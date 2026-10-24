package service

import (
	"fmt"
	"slices"
	"strings"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/resolver"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
)

type LinkService struct {
	cards     store.CardStore
	boards    store.BoardStore
	elsewhere *Elsewhere
	project   store.ProjectStore
}

func NewLinkService(cards store.CardStore, boards store.BoardStore) *LinkService {
	return &LinkService{cards: cards, boards: boards}
}

// SetElsewhere lets links reach boards in other projects. project is the
// current project's config, where links to them are recorded.
func (s *LinkService) SetElsewhere(e *Elsewhere, project store.ProjectStore) {
	s.elsewhere, s.project = e, project
}

func (s *LinkService) loadGraph() (*LinkGraph, error) {
	g, err := LoadLinkGraph(s.boards, s.cards)
	if err != nil {
		return nil, err
	}
	g.SetElsewhere(s.elsewhere)
	return g, nil
}

func (s *LinkService) Link(boardName, cardRef string, blockedBy, related []string) (*model.Card, error) {
	g, err := s.loadGraph()
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
	if err := s.RecordLinkedBoards(g, boardName, &card); err != nil {
		return nil, err
	}
	card.UpdatedAtMillis = util.NowMillis()
	if err := s.cards.Update(boardName, &card); err != nil {
		return nil, err
	}
	return &card, nil
}

// RecordLinkedBoards records, in the current project's [linked_boards], each
// board in another project that the card links to: its project and board
// name, and where its repo can be fetched from. That only improves the
// message when the board later isn't on this machine. Callers run it before
// writing the card, so a failed save leaves the card unchanged.
func (s *LinkService) RecordLinkedBoards(g *LinkGraph, boardName string, card *model.Card) error {
	if s.elsewhere == nil || s.project == nil {
		return nil
	}
	found := map[string]model.LinkedBoard{}
	for _, ref := range slices.Concat(card.BlockedBy, card.Related) {
		if !strings.Contains(ref, ":") {
			continue
		}
		target := g.Resolve(boardName, ref)
		b := g.boards[target.board]
		if b == nil || b.project == "" || b.config == nil {
			continue
		}
		linked := model.LinkedBoard{Name: registry.ProjectName(b.project, b.dataLocation) + "/" + b.config.Name}
		if s.elsewhere.Remote != nil {
			linked.Remote = s.elsewhere.Remote(b.project)
		}
		found[b.config.ID] = linked
	}
	if len(found) == 0 {
		return nil
	}

	cfg, err := s.project.Load()
	if err != nil {
		return fmt.Errorf("recording linked boards: %w", err)
	}
	changed := false
	for id, linked := range found {
		if cfg.LinkedBoards[id] != linked {
			if cfg.LinkedBoards == nil {
				cfg.LinkedBoards = map[string]model.LinkedBoard{}
			}
			cfg.LinkedBoards[id] = linked
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := s.project.Save(cfg); err != nil {
		return fmt.Errorf("recording linked boards: %w", err)
	}
	return nil
}

// storedRef resolves a link argument to what a card stores. A bare ref
// names a card on the card's own board. A board:card ref names a board by
// name or ID in the current project, or by ID in another registered
// project, and a card on it by ID, alias or fuzzy match. A card on the
// card's own board is stored as its bare ID, any other as boardID:cardID.
func (s *LinkService) storedRef(g *LinkGraph, boardName, ref string) (string, error) {
	boardRef, cardRef, qualified := strings.Cut(ref, ":")
	if !qualified {
		boardRef, cardRef = boardName, ref
	} else if boardRef == "" || cardRef == "" {
		return "", kanerr.InvalidField("link", "expected board:card, got "+ref)
	}
	key, err := g.boardForRef(boardRef)
	if err != nil {
		return "", err
	}
	cards, err := g.Cards(key)
	if err != nil {
		return "", err
	}
	target, err := resolver.ResolveInCards(cards, cardRef)
	if err != nil {
		return "", err
	}
	if key == boardName {
		return target.ID, nil
	}
	return g.boards[key].config.ID + ":" + target.ID, nil
}

// Prepare validates all requested links before changing a candidate card.
func (s *LinkService) Prepare(g *LinkGraph, boardName string, card *model.Card, blockedBy, related []string) error {
	if _, err := g.Cards(boardName); err != nil {
		return err
	}
	resolve := func(refs []string, existing []string) ([]string, error) {
		result := slices.Clone(existing)
		for _, ref := range refs {
			stored, err := s.storedRef(g, boardName, ref)
			if err != nil {
				return nil, fmt.Errorf("cannot link to %s: %w", ref, err)
			}
			if !slices.Contains(result, stored) {
				result = append(result, stored)
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
		if blocker.State == LinkFound && blocker.board == boardName && ancestors[blocker.Card.ID] {
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
		if isCandidate || (target.State == LinkFound && target.board == boardName && target.Card.ID == card.ID) {
			return kanerr.InvalidField("blocked_by", "blocker cycle: "+strings.Join(append(path, card.ID), " -> "))
		}
		if target.State != LinkFound {
			return nil
		}
		key := target.board + "\x00" + target.Card.ID
		if seen[key] {
			return nil
		}
		seen[key] = true
		path = append(path, target.Card.ID)
		for _, next := range target.Card.BlockedBy {
			if err := walk(target.board, next, path); err != nil {
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
	g, err := s.loadGraph()
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
			if stored, err := s.storedRef(g, boardName, ref); err == nil {
				remove[stored] = true
			}
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
