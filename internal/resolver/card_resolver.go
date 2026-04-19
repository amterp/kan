package resolver

import (
	stderrors "errors"
	"sort"
	"strings"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// MaxBoardsForCrossSearch is the maximum number of boards to search across
// when no board is specified. Beyond this, the user must specify a board
// explicitly or set a default. Keeps lookups fast in repos with many boards.
const MaxBoardsForCrossSearch = 10

// Fuzzy-matching tunables. minFuzzyQueryLen stops single-letter queries from
// returning half the board as "ambiguous"; maxAmbiguousResults caps the list
// shown to the user when there are many matches.
const (
	minFuzzyQueryLen    = 3
	maxAmbiguousResults = 5
)

// CrossBoardMatch holds the result of resolving a card across multiple boards.
type CrossBoardMatch struct {
	Card      *model.Card
	BoardName string
}

// CardResolver handles card ID and alias resolution.
type CardResolver struct {
	cardStore store.CardStore
}

// NewCardResolver creates a new card resolver.
func NewCardResolver(cardStore store.CardStore) *CardResolver {
	return &CardResolver{cardStore: cardStore}
}

// Resolve finds a card by ID or alias within a single board.
// Lookup order: exact ID -> exact alias -> case-insensitive substring fuzzy
// match against alias/ID. A single fuzzy match resolves; multiple matches
// produce AmbiguousCardError. Fuzzy is skipped for queries shorter than
// minFuzzyQueryLen to avoid noisy ambiguity on tiny inputs.
func (r *CardResolver) Resolve(boardName, idOrAlias string) (*model.Card, error) {
	// 1. Exact ID (fast path, avoids a full List).
	if card, err := r.cardStore.Get(boardName, idOrAlias); err == nil {
		return card, nil
	}

	// 2. Load the board once for alias-exact + fuzzy.
	cards, err := r.cardStore.List(boardName)
	if err != nil {
		return nil, err
	}

	// 3. Exact alias.
	for _, c := range cards {
		if c.Alias != "" && c.Alias == idOrAlias {
			return c, nil
		}
	}

	// 4. Fuzzy fallback.
	if len(idOrAlias) < minFuzzyQueryLen {
		return nil, kanerr.CardNotFound(idOrAlias)
	}
	matches := fuzzyMatchCards(cards, idOrAlias)
	switch len(matches) {
	case 0:
		return nil, kanerr.CardNotFound(idOrAlias)
	case 1:
		return matches[0], nil
	default:
		return nil, kanerr.NewAmbiguousCardError(idOrAlias, toAmbiguousMatches(matches), maxAmbiguousResults)
	}
}

// ResolveAcrossBoards searches for a card by ID or alias across multiple boards.
// Returns all matches found. Callers decide how to handle 0, 1, or N results.
//
// Ambiguous fuzzy matches within a single board are flattened into multiple
// CrossBoardMatch entries so the cross-board aggregator produces one coherent
// "multiple matches" UX regardless of where the ambiguity came from.
func (r *CardResolver) ResolveAcrossBoards(boards []string, idOrAlias string) ([]CrossBoardMatch, error) {
	var matches []CrossBoardMatch
	for _, board := range boards {
		card, err := r.Resolve(board, idOrAlias)
		if err != nil {
			if kanerr.IsNotFound(err) {
				continue
			}
			var ambig *kanerr.AmbiguousCardError
			if stderrors.As(err, &ambig) {
				for _, m := range ambig.Matches {
					if c, e := r.cardStore.Get(board, m.ID); e == nil {
						matches = append(matches, CrossBoardMatch{Card: c, BoardName: board})
					}
				}
				continue
			}
			return nil, err
		}
		matches = append(matches, CrossBoardMatch{Card: card, BoardName: board})
	}
	return matches, nil
}

// fuzzyMatchCards returns cards whose alias or ID contains the query
// (case-insensitive). Results are sorted: prefix matches (alias or ID starting
// with the query) come first, then alphabetically by alias, with ID as
// tiebreaker. This keeps the disambiguation list predictable.
func fuzzyMatchCards(cards []*model.Card, query string) []*model.Card {
	q := strings.ToLower(query)
	type scored struct {
		card     *model.Card
		isPrefix bool
	}
	var hits []scored
	for _, c := range cards {
		alias := strings.ToLower(c.Alias)
		id := strings.ToLower(c.ID)
		aliasMatch := c.Alias != "" && strings.Contains(alias, q)
		idMatch := strings.Contains(id, q)
		if !aliasMatch && !idMatch {
			continue
		}
		isPrefix := (c.Alias != "" && strings.HasPrefix(alias, q)) || strings.HasPrefix(id, q)
		hits = append(hits, scored{card: c, isPrefix: isPrefix})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].isPrefix != hits[j].isPrefix {
			return hits[i].isPrefix
		}
		ai, aj := hits[i].card.Alias, hits[j].card.Alias
		if ai != aj {
			return ai < aj
		}
		return hits[i].card.ID < hits[j].card.ID
	})
	out := make([]*model.Card, len(hits))
	for i, h := range hits {
		out[i] = h.card
	}
	return out
}

func toAmbiguousMatches(cards []*model.Card) []kanerr.AmbiguousMatch {
	out := make([]kanerr.AmbiguousMatch, len(cards))
	for i, c := range cards {
		out[i] = kanerr.AmbiguousMatch{Alias: c.Alias, ID: c.ID}
	}
	return out
}
