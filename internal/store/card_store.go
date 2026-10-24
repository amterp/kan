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
	"github.com/amterp/kan/internal/version"
)

// FileCardStore implements CardStore using the filesystem.
type FileCardStore struct {
	fileAccess
	paths *config.Paths
}

// NewCardStore creates a new card store.
func NewCardStore(paths *config.Paths) *FileCardStore {
	return &FileCardStore{paths: paths}
}

// NewReadOnlyCardStore creates a card store that reads through overlay
// (which may be nil) and refuses to write.
func NewReadOnlyCardStore(paths *config.Paths, overlay *Overlay) *FileCardStore {
	return &FileCardStore{paths: paths, fileAccess: fileAccess{overlay: overlay, readOnly: true}}
}

// Create writes a new card to disk.
func (s *FileCardStore) Create(boardName string, card *model.Card) error {
	path := s.paths.CardPath(boardName, card.ID)
	if err := s.checkWritable(path); err != nil {
		return err
	}

	// Ensure cards directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cards directory: %w", err)
	}

	return s.writeCard(path, card, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
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
	if err := s.checkWritable(path); err != nil {
		return err
	}
	if err := s.writeCard(path, card, os.O_WRONLY|os.O_CREATE|os.O_TRUNC); err != nil {
		return fmt.Errorf("failed to update card %s: %w", card.ID, err)
	}
	return nil
}

// Delete removes a card from disk.
func (s *FileCardStore) Delete(boardName, cardID string) error {
	path := s.paths.CardPath(boardName, cardID)
	if err := s.checkWritable(path); err != nil {
		return err
	}
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
	listing, err := s.ListWithDiagnostics(boardName)
	for _, failure := range listing.Failures {
		fmt.Fprintf(os.Stderr, "Warning: skipping malformed card file %s.json: %v\n", failure.CardID, failure.Err)
	}
	return listing.Cards, err
}

func (s *FileCardStore) ListWithDiagnostics(boardName string) (CardListing, error) {
	listing := CardListing{Cards: []*model.Card{}}
	cardsDir := s.paths.CardsDir(boardName)

	entries, err := os.ReadDir(cardsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return listing, nil
		}
		return listing, fmt.Errorf("failed to read cards directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(cardsDir, entry.Name())
		card, err := s.readCard(path)
		if err != nil {
			listing.Failures = append(listing.Failures, CardReadFailure{
				CardID: strings.TrimSuffix(entry.Name(), ".json"), Err: err,
			})
			continue
		}
		listing.Cards = append(listing.Cards, card)
	}

	return listing, nil
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
	data, err := s.readFile(path)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Version int `json:"_v"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Strict version validation
	if envelope.Version == 0 {
		return nil, version.MissingCardVersion(path)
	}
	if envelope.Version != version.CurrentCardVersion {
		return nil, version.InvalidCardVersion(path, envelope.Version, version.CurrentCardVersion)
	}
	var card model.Card
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, fmt.Errorf("invalid card JSON: %w", err)
	}

	return &card, nil
}

func (s *FileCardStore) writeCard(path string, card *model.Card, flags int) error {
	// Stamp current schema version
	card.Version = version.CurrentCardVersion

	// MarshalFile keeps the card pretty-printed but collapses each history
	// entry onto a single line, so appending a transition is a one-line diff.
	data, err := card.MarshalFile()
	if err != nil {
		return fmt.Errorf("failed to marshal card: %w", err)
	}

	file, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return fmt.Errorf("failed to open card file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write card file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close card file: %w", err)
	}
	return nil
}
