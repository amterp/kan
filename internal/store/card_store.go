package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
)

// FileCardStore implements CardStore using the filesystem.
type FileCardStore struct {
	paths *config.Paths
}

// NewCardStore creates a new card store.
func NewCardStore(paths *config.Paths) *FileCardStore {
	return &FileCardStore{paths: paths}
}

// Create writes a new card to disk.
func (s *FileCardStore) Create(boardName string, card *model.Card) error {
	path := s.paths.CardPath(boardName, card.ID)

	// Ensure cards directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cards directory: %w", err)
	}

	return s.writeCard(path, card)
}

// Get reads a card from disk by ID.
func (s *FileCardStore) Get(boardName, cardID string) (*model.Card, error) {
	path := s.paths.CardPath(boardName, cardID)
	card, err := s.readCard(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, kanerr.CardNotFound(cardID)
		}
		return nil, fmt.Errorf("failed to read card %s: %w", cardID, err)
	}
	return card, nil
}

// Update writes an existing card to disk.
func (s *FileCardStore) Update(boardName string, card *model.Card) error {
	path := s.paths.CardPath(boardName, card.ID)
	if err := s.writeCard(path, card); err != nil {
		return fmt.Errorf("failed to update card %s: %w", card.ID, err)
	}
	return nil
}

// Delete removes a card from disk.
func (s *FileCardStore) Delete(boardName, cardID string) error {
	path := s.paths.CardPath(boardName, cardID)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return kanerr.CardNotFound(cardID)
		}
		return fmt.Errorf("failed to delete card %s: %w", cardID, err)
	}
	return nil
}

// List returns all cards for a board.
// Malformed card files are logged and skipped.
func (s *FileCardStore) List(boardName string) ([]*model.Card, error) {
	cardsDir := s.paths.CardsDir(boardName)

	entries, err := os.ReadDir(cardsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*model.Card{}, nil // Return empty slice, not nil
		}
		return nil, fmt.Errorf("failed to read cards directory: %w", err)
	}

	var cards []*model.Card
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(cardsDir, entry.Name())
		card, err := s.readCard(path)
		if err != nil {
			// Log warning but don't fail - allows partial reads
			fmt.Fprintf(os.Stderr, "Warning: skipping malformed card file %s: %v\n", entry.Name(), err)
			continue
		}
		cards = append(cards, card)
	}

	if cards == nil {
		cards = []*model.Card{} // Ensure non-nil
	}
	return cards, nil
}

// FindByAlias searches for a card by alias.
func (s *FileCardStore) FindByAlias(boardName, alias string) (*model.Card, error) {
	cards, err := s.List(boardName)
	if err != nil {
		return nil, err
	}

	for _, card := range cards {
		if card.Alias == alias {
			return card, nil
		}
	}

	return nil, kanerr.CardNotFound(alias)
}

func (s *FileCardStore) readCard(path string) (*model.Card, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var card model.Card
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &card, nil
}

func (s *FileCardStore) writeCard(path string, card *model.Card) error {
	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal card: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write card file: %w", err)
	}
	return nil
}
