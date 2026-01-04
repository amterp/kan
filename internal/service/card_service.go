package service

import (
	fid "github.com/amterp/flexid"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
)

// CardService handles card operations.
type CardService struct {
	cardStore    store.CardStore
	boardStore   store.BoardStore
	aliasService *AliasService
}

// NewCardService creates a new card service.
func NewCardService(cardStore store.CardStore, boardStore store.BoardStore, aliasService *AliasService) *CardService {
	return &CardService{
		cardStore:    cardStore,
		boardStore:   boardStore,
		aliasService: aliasService,
	}
}

// AddCardInput contains the input for adding a card.
type AddCardInput struct {
	BoardName   string
	Title       string
	Description string
	Column      string
	Labels      []string
	Parent      string
	Creator     string
}

// Add creates a new card.
func (s *CardService) Add(input AddCardInput) (*model.Card, error) {
	// Get board config
	boardCfg, err := s.boardStore.Get(input.BoardName)
	if err != nil {
		return nil, err // Already wrapped with proper error type by store
	}

	// Determine column
	column := input.Column
	if column == "" {
		column = boardCfg.GetDefaultColumn()
	}
	if !boardCfg.HasColumn(column) {
		return nil, kanerr.ColumnNotFound(column, input.BoardName)
	}

	// Validate labels
	for _, label := range input.Labels {
		if !boardCfg.HasLabel(label) {
			return nil, kanerr.LabelNotFound(label, input.BoardName)
		}
	}

	// Generate ID and alias
	id := fid.MustGenerate()
	alias, err := s.aliasService.GenerateAlias(input.BoardName, input.Title)
	if err != nil {
		return nil, err
	}

	now := util.NowMillis()
	card := &model.Card{
		ID:              id,
		Alias:           alias,
		AliasExplicit:   false,
		Title:           input.Title,
		Description:     input.Description,
		Column:          column,
		Labels:          input.Labels,
		Parent:          input.Parent,
		Creator:         input.Creator,
		CreatedAtMillis: now,
		UpdatedAtMillis: now,
	}

	if err := s.cardStore.Create(input.BoardName, card); err != nil {
		return nil, err
	}

	return card, nil
}

// Get retrieves a card by ID.
func (s *CardService) Get(boardName, cardID string) (*model.Card, error) {
	return s.cardStore.Get(boardName, cardID)
}

// Update saves changes to a card.
func (s *CardService) Update(boardName string, card *model.Card) error {
	card.UpdatedAtMillis = util.NowMillis()
	return s.cardStore.Update(boardName, card)
}

// List returns all cards for a board, optionally filtered by column.
func (s *CardService) List(boardName string, columnFilter string) ([]*model.Card, error) {
	cards, err := s.cardStore.List(boardName)
	if err != nil {
		return nil, err
	}

	if columnFilter == "" {
		return cards, nil
	}

	// Filter by column
	filtered := make([]*model.Card, 0)
	for _, card := range cards {
		if card.Column == columnFilter {
			filtered = append(filtered, card)
		}
	}

	return filtered, nil
}

// FindByIDOrAlias finds a card by ID or alias.
func (s *CardService) FindByIDOrAlias(boardName, idOrAlias string) (*model.Card, error) {
	// Try ID first
	card, err := s.cardStore.Get(boardName, idOrAlias)
	if err == nil {
		return card, nil
	}

	// Try alias
	return s.cardStore.FindByAlias(boardName, idOrAlias)
}

// UpdateTitle updates the card title and regenerates alias if not explicit.
func (s *CardService) UpdateTitle(boardName string, card *model.Card, newTitle string) error {
	card.Title = newTitle

	if !card.AliasExplicit {
		alias, err := s.aliasService.GenerateAlias(boardName, newTitle)
		if err != nil {
			return err
		}
		card.Alias = alias
	}

	return s.Update(boardName, card)
}
