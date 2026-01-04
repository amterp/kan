package service

import (
	"fmt"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
)

// AliasService handles alias generation and collision detection.
type AliasService struct {
	cardStore store.CardStore
}

// NewAliasService creates a new alias service.
func NewAliasService(cardStore store.CardStore) *AliasService {
	return &AliasService{cardStore: cardStore}
}

// GenerateAlias creates a unique alias from a title.
// If the base alias is taken, appends -2, -3, etc.
func (s *AliasService) GenerateAlias(boardName, title string) (string, error) {
	base := util.Slugify(title)
	if base == "" {
		base = "card"
	}

	// Check if base alias is available
	if s.IsAliasAvailable(boardName, base) {
		return base, nil
	}

	// Find next available suffix
	for i := 2; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if s.IsAliasAvailable(boardName, candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not generate unique alias for %q", title)
}

// IsAliasAvailable returns true if the alias is not in use.
func (s *AliasService) IsAliasAvailable(boardName, alias string) bool {
	_, err := s.cardStore.FindByAlias(boardName, alias)
	return kanerr.IsNotFound(err)
}
