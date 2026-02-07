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
func (s *AliasService) GenerateAlias(boardName, title string) (string, error) {
	words := util.SlugWords(title)
	if len(words) == 0 {
		words = []string{"card"}
	}

	initialCount := wordsForThreshold(words)
	base := strings.Join(words[:initialCount], "-")

	if s.IsAliasAvailable(boardName, base) {
		return base, nil
	}

	// Collision: try adding one more title word at a time
	for i := initialCount; i < len(words); i++ {
		candidate := strings.Join(words[:i+1], "-")
		if s.IsAliasAvailable(boardName, candidate) {
			return candidate, nil
		}
	}

	// All title words exhausted: fall back to numeric suffix on the base slug
	for i := 2; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if s.IsAliasAvailable(boardName, candidate) {
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
func (s *AliasService) IsAliasAvailable(boardName, alias string) bool {
	_, err := s.cardStore.FindByAlias(boardName, alias)
	return kanerr.IsNotFound(err)
}
