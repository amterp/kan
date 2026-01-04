package resolver

import (
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

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
