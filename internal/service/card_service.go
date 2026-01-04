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
		Column:          column, // Still store for backward compat, but authoritative source is board config
		Labels:          input.Labels,
		Parent:          input.Parent,
		Creator:         input.Creator,
		CreatedAtMillis: now,
		UpdatedAtMillis: now,
	}

	if err := s.cardStore.Create(input.BoardName, card); err != nil {
		return nil, err
	}

	// Add card to column's card list and save board config
	boardCfg.AddCardToColumn(id, column)
	if err := s.boardStore.Update(boardCfg); err != nil {
		// Card was created but board config update failed - log but don't fail
		// The card's column field will still be correct
		return card, nil
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
// Cards are returned in the order specified by the board's column card_ids.
func (s *CardService) List(boardName string, columnFilter string) ([]*model.Card, error) {
	cards, err := s.cardStore.List(boardName)
	if err != nil {
		return nil, err
	}

	// Get board config to determine card ordering and column membership
	boardCfg, err := s.boardStore.Get(boardName)
	if err != nil {
		// Fall back to unordered if board config can't be read
		if columnFilter == "" {
			return cards, nil
		}
		filtered := make([]*model.Card, 0)
		for _, card := range cards {
			if card.Column == columnFilter {
				filtered = append(filtered, card)
			}
		}
		return filtered, nil
	}

	// Build card ID to card map for quick lookup
	cardMap := make(map[string]*model.Card)
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	// Build ordered result based on column card_ids
	var result []*model.Card
	for _, col := range boardCfg.Columns {
		if columnFilter != "" && col.Name != columnFilter {
			continue
		}
		for _, cardID := range col.CardIDs {
			if card, ok := cardMap[cardID]; ok {
				// Update card's column from board config (authoritative source)
				card.Column = col.Name
				result = append(result, card)
				delete(cardMap, cardID) // Mark as processed
			}
		}
	}

	// Append any cards not in column card_ids (orphaned or legacy cards)
	// These are cards that exist but aren't tracked in the board config yet
	for _, card := range cardMap {
		if columnFilter == "" || card.Column == columnFilter {
			result = append(result, card)
		}
	}

	if result == nil {
		result = []*model.Card{}
	}
	return result, nil
}

// MoveCard moves a card to a different column at the end.
func (s *CardService) MoveCard(boardName, cardID, targetColumn string) error {
	return s.MoveCardAt(boardName, cardID, targetColumn, -1)
}

// MoveCardAt moves a card to a different column at a specific position.
// If position is -1, appends to end.
func (s *CardService) MoveCardAt(boardName, cardID, targetColumn string, position int) error {
	// Get board config
	boardCfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return err
	}

	// Validate target column exists
	if !boardCfg.HasColumn(targetColumn) {
		return kanerr.ColumnNotFound(targetColumn, boardName)
	}

	// Get the card to verify it exists and update its column field
	card, err := s.cardStore.Get(boardName, cardID)
	if err != nil {
		return err
	}

	// Update card's column field (for backward compat)
	card.Column = targetColumn
	card.UpdatedAtMillis = util.NowMillis()
	if err := s.cardStore.Update(boardName, card); err != nil {
		return err
	}

	// Move card in board config at position
	boardCfg.MoveCardToColumnAt(cardID, targetColumn, position)
	return s.boardStore.Update(boardCfg)
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
