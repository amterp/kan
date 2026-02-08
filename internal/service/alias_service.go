package service

import (
	"fmt"
	"strings"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
)

const (
	// slugThreshold is the max character length we aim for when building the
	// initial slug. We'll include at least minSlugWords, then keep adding
	// words while the joined result stays within this budget.
	slugThreshold = 20
	minSlugWords  = 2
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
// It builds a short slug progressively: start with a threshold-limited set of
// words, then on collision add more title words before falling back to -N.
// excludeCardID allows excluding a specific card from collision detection,
// useful when regenerating alias for an existing card.
func (s *AliasService) GenerateAlias(boardName, title, excludeCardID string) (string, error) {
	words := util.SlugWords(title)
	if len(words) == 0 {
		words = []string{"card"}
	}

	initialCount := wordsForThreshold(words)
	base := strings.Join(words[:initialCount], "-")

	if s.IsAliasAvailable(boardName, base, excludeCardID) {
		return base, nil
	}

	// Collision: try adding one more title word at a time
	for i := initialCount; i < len(words); i++ {
		candidate := strings.Join(words[:i+1], "-")
		if s.IsAliasAvailable(boardName, candidate, excludeCardID) {
			return candidate, nil
		}
	}

	// All title words exhausted: fall back to numeric suffix on the base slug
	for i := 2; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if s.IsAliasAvailable(boardName, candidate, excludeCardID) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not generate unique alias for %q", title)
}

// wordsForThreshold returns how many words to include in the initial slug.
// Always includes at least minSlugWords (if available), then adds words while
// the joined length stays within slugThreshold.
func wordsForThreshold(words []string) int {
	count := min(minSlugWords, len(words))

	for i := count; i < len(words); i++ {
		// length of joined slug if we include words[i]:
		// current joined length + hyphen + next word
		candidateLen := joinedLen(words[:i+1])
		if candidateLen > slugThreshold {
			break
		}
		count = i + 1
	}

	return count
}

// joinedLen returns the length of words joined by hyphens, without allocating.
func joinedLen(words []string) int {
	if len(words) == 0 {
		return 0
	}
	n := len(words) - 1 // hyphens
	for _, w := range words {
		n += len(w)
	}
	return n
}

// IsAliasAvailable returns true if the alias is not in use.
// excludeCardID allows excluding a specific card from the check - useful when
// regenerating alias for an existing card to avoid self-collision.
func (s *AliasService) IsAliasAvailable(boardName, alias, excludeCardID string) bool {
	card, err := s.cardStore.FindByAlias(boardName, alias)
	if kanerr.IsNotFound(err) {
		return true
	}
	if err != nil {
		return false
	}
	// If we found a card, check if it's the one we're excluding
	return excludeCardID != "" && card.ID == excludeCardID
}
