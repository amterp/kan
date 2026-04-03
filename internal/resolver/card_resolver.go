package resolver

import (
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// MaxBoardsForCrossSearch is the maximum number of boards to search across
// when no board is specified. Beyond this, the user must specify a board
// explicitly or set a default. Keeps lookups fast in repos with many boards.
const MaxBoardsForCrossSearch = 10

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

// Resolve finds a card by ID or alias.
// Tries exact ID match first (faster), then falls back to alias lookup.
func (r *CardResolver) Resolve(boardName, idOrAlias string) (*model.Card, error) {
	// Try direct ID lookup first
	card, err := r.cardStore.Get(boardName, idOrAlias)
	if err == nil {
		return card, nil
	}

	// Fall back to alias lookup
	card, err = r.cardStore.FindByAlias(boardName, idOrAlias)
	if err == nil {
		return card, nil
	}

	// Return not-found error (store already wraps with proper type)
	if kanerr.IsNotFound(err) {
		return nil, kanerr.CardNotFound(idOrAlias)
	}
	return nil, err
}

// ResolveAcrossBoards searches for a card by ID or alias across multiple boards.
// Returns all matches found. Callers decide how to handle 0, 1, or N results.
func (r *CardResolver) ResolveAcrossBoards(boards []string, idOrAlias string) ([]CrossBoardMatch, error) {
	var matches []CrossBoardMatch
	for _, board := range boards {
		card, err := r.Resolve(board, idOrAlias)
		if err != nil {
			if kanerr.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		matches = append(matches, CrossBoardMatch{Card: card, BoardName: board})
	}
	return matches, nil
}
