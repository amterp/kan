package service

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/id"

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
	hookService  *HookService
}

// NewCardService creates a new card service.
func NewCardService(cardStore store.CardStore, boardStore store.BoardStore, aliasService *AliasService) *CardService {
	return &CardService{
		cardStore:    cardStore,
		boardStore:   boardStore,
		aliasService: aliasService,
	}
}

// SetHookService sets the hook service for executing pattern hooks.
// When set, hooks matching card titles will be executed after card creation.
func (s *CardService) SetHookService(hookService *HookService) {
	s.hookService = hookService
}

// AddCardInput contains the input for adding a card.
type AddCardInput struct {
	BoardName    string
	Title        string
	Description  string
	Column       string
	Parent       string
	Creator      string
	CustomFields map[string]string // custom fields to set (parsed from key=value)
}

// EditCardInput contains the input for editing a card.
// Pointer fields indicate "set this field"; nil means "don't change".
type EditCardInput struct {
	BoardName     string
	CardIDOrAlias string
	Title         *string           // nil = no change
	Description   *string           // nil = no change
	Column        *string           // nil = no change
	Parent        *string           // nil = no change, empty string = clear parent
	Alias         *string           // nil = no change
	CustomFields  map[string]string // fields to set/update (parsed from key=value)
}

// Add creates a new card.
// Returns the created card, any hook results (may be nil), and an error.
// Hooks are executed after the card is fully persisted. Hook failures are non-fatal
// and reported in the results rather than as errors.
func (s *CardService) Add(input AddCardInput) (*model.Card, []*HookResult, error) {
	// Get board config
	boardCfg, err := s.boardStore.Get(input.BoardName)
	if err != nil {
		return nil, nil, err // Already wrapped with proper error type by store
	}

	// Determine column
	column := input.Column
	if column == "" {
		column = boardCfg.GetDefaultColumn()
	}
	if !boardCfg.HasColumn(column) {
		return nil, nil, kanerr.ColumnNotFound(column, input.BoardName)
	}

	// Generate ID and alias
	cardID := id.Generate(id.Card)
	alias, err := s.aliasService.GenerateAlias(input.BoardName, input.Title)
	if err != nil {
		return nil, nil, err
	}

	now := util.NowMillis()
	card := &model.Card{
		ID:              cardID,
		Alias:           alias,
		AliasExplicit:   false,
		Title:           input.Title,
		Description:     input.Description,
		Parent:          input.Parent,
		Creator:         input.Creator,
		CreatedAtMillis: now,
		UpdatedAtMillis: now,
	}

	// Apply custom fields if provided
	if len(input.CustomFields) > 0 {
		if err := s.validateAndApplyCustomFields(card, boardCfg, input.CustomFields); err != nil {
			return nil, nil, err
		}
	}

	if err := s.cardStore.Create(input.BoardName, card); err != nil {
		return nil, nil, err
	}

	// Add card to column's card list and save board config
	boardCfg.AddCardToColumn(cardID, column)
	if err := s.boardStore.Update(boardCfg); err != nil {
		// Card was created but board config update failed - log but don't fail
		card.Column = column // Populate for return (not persisted)
		return card, nil, nil
	}

	// Populate Column for return (computed, not persisted)
	card.Column = column

	// Execute pattern hooks if configured
	var hookResults []*HookResult
	if s.hookService != nil && len(boardCfg.PatternHooks) > 0 {
		matchingHooks := s.hookService.FindMatchingHooks(boardCfg.PatternHooks, input.Title)
		if len(matchingHooks) > 0 {
			hookResults = s.hookService.ExecuteHooks(matchingHooks, cardID, input.BoardName)
		}
	}

	return card, hookResults, nil
}

// Get retrieves a card by ID.
func (s *CardService) Get(boardName, cardID string) (*model.Card, error) {
	return s.cardStore.Get(boardName, cardID)
}

// Update saves changes to a card.
func (s *CardService) Update(boardName string, card *model.Card) error {
	// Validate custom fields don't use reserved prefixes
	if err := model.ValidateCustomFields(card.CustomFields); err != nil {
		return err
	}

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
		// Without board config, we can't determine column membership or ordering.
		// Return unordered cards without column filtering.
		return cards, nil
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

	// Append any cards not in column card_ids (orphaned cards)
	// These are cards that exist but aren't tracked in the board config.
	// Only include them if no column filter is specified (since they have no column).
	if columnFilter == "" {
		for _, card := range cardMap {
			result = append(result, card)
		}
	}

	if result == nil {
		result = []*model.Card{}
	}
	return result, nil
}

// MoveCard moves a card to a different column at the top.
func (s *CardService) MoveCard(boardName, cardID, targetColumn string) error {
	return s.MoveCardAt(boardName, cardID, targetColumn, 0)
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

	// Verify the card exists
	_, err = s.cardStore.Get(boardName, cardID)
	if err != nil {
		return err
	}

	// Move card in board config (authoritative source for column membership)
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

// Edit applies changes specified in the input to an existing card.
func (s *CardService) Edit(input EditCardInput) (*model.Card, error) {
	// Resolve card
	card, err := s.FindByIDOrAlias(input.BoardName, input.CardIDOrAlias)
	if err != nil {
		return nil, err
	}

	// Get board config for validation
	boardCfg, err := s.boardStore.Get(input.BoardName)
	if err != nil {
		return nil, err
	}

	needsUpdate := false

	// Handle title change (regenerates alias if not explicit)
	if input.Title != nil {
		if *input.Title == "" {
			return nil, kanerr.InvalidField("title", "cannot be empty")
		}
		if err := s.UpdateTitle(input.BoardName, card, *input.Title); err != nil {
			return nil, err
		}
		// Re-fetch card after title update (alias may have changed)
		card, err = s.Get(input.BoardName, card.ID)
		if err != nil {
			return nil, err
		}
	}

	// Handle column change
	if input.Column != nil {
		if !boardCfg.HasColumn(*input.Column) {
			return nil, kanerr.ColumnNotFound(*input.Column, input.BoardName)
		}
		if err := s.MoveCard(input.BoardName, card.ID, *input.Column); err != nil {
			return nil, err
		}
		card.Column = *input.Column
	}

	// Handle description change
	if input.Description != nil {
		card.Description = *input.Description
		needsUpdate = true
	}

	// Handle parent change
	if input.Parent != nil {
		if *input.Parent != "" {
			// Validate parent exists
			_, err := s.FindByIDOrAlias(input.BoardName, *input.Parent)
			if err != nil {
				return nil, fmt.Errorf("parent card not found: %s", *input.Parent)
			}
		}
		card.Parent = *input.Parent
		needsUpdate = true
	}

	// Handle explicit alias change
	if input.Alias != nil {
		if *input.Alias == "" {
			return nil, kanerr.InvalidField("alias", "cannot be empty")
		}
		// Check alias not already in use by another card
		existing, err := s.cardStore.FindByAlias(input.BoardName, *input.Alias)
		if err == nil && existing.ID != card.ID {
			return nil, kanerr.InvalidField("alias", fmt.Sprintf("already in use by card %s", existing.ID))
		}
		card.Alias = *input.Alias
		card.AliasExplicit = true
		needsUpdate = true
	}

	// Handle custom fields
	if len(input.CustomFields) > 0 {
		if err := s.validateAndApplyCustomFields(card, boardCfg, input.CustomFields); err != nil {
			return nil, err
		}
		needsUpdate = true
	}

	if needsUpdate {
		if err := s.Update(input.BoardName, card); err != nil {
			return nil, err
		}
	}

	return card, nil
}

// validateAndApplyCustomFields validates and applies custom field changes.
func (s *CardService) validateAndApplyCustomFields(card *model.Card, boardCfg *model.BoardConfig, fields map[string]string) error {
	if card.CustomFields == nil {
		card.CustomFields = make(map[string]any)
	}

	for key, value := range fields {
		// Validate field name doesn't use reserved prefix
		if err := model.ValidateCustomFieldName(key); err != nil {
			return err
		}

		// Check field is defined in board config
		schema, exists := boardCfg.CustomFields[key]
		if !exists {
			return kanerr.InvalidField("field", fmt.Sprintf("%q is not defined in board config", key))
		}

		// Validate value against schema
		switch schema.Type {
		case model.FieldTypeEnum:
			if !isValidOption(schema.Options, value) {
				return kanerr.InvalidField(key, fmt.Sprintf("must be one of: %s", formatOptions(schema.Options)))
			}
			card.CustomFields[key] = value

		case model.FieldTypeEnumSet:
			// Parse comma-separated values, validate against options
			vals := parseSetValues(value)
			vals = dedup(vals)
			if len(vals) > model.MaxSetItems {
				return kanerr.InvalidField(key, fmt.Sprintf("too many values (max %d)", model.MaxSetItems))
			}
			for _, v := range vals {
				if !isValidOption(schema.Options, v) {
					return kanerr.InvalidField(key, fmt.Sprintf("%q is not a valid option; must be one of: %s", v, formatOptions(schema.Options)))
				}
			}
			card.CustomFields[key] = vals

		case model.FieldTypeFreeSet:
			// Parse comma-separated values, no option validation
			vals := parseSetValues(value)
			vals = dedup(vals)
			if len(vals) > model.MaxSetItems {
				return kanerr.InvalidField(key, fmt.Sprintf("too many values (max %d)", model.MaxSetItems))
			}
			card.CustomFields[key] = vals

		case model.FieldTypeString, model.FieldTypeDate:
			card.CustomFields[key] = value

		default:
			return kanerr.InvalidField(key, fmt.Sprintf("unknown field type %q", schema.Type))
		}
	}

	return nil
}

// Delete removes a card from the board.
func (s *CardService) Delete(boardName, cardID string) error {
	// Remove from card store
	if err := s.cardStore.Delete(boardName, cardID); err != nil {
		return err
	}

	// Remove from board config
	boardCfg, err := s.boardStore.Get(boardName)
	if err != nil {
		// Card is deleted, board config update failure is non-fatal
		return nil
	}

	boardCfg.RemoveCardFromColumn(cardID)
	return s.boardStore.Update(boardCfg)
}

// isValidOption checks if a value exists in the options list.
func isValidOption(options []model.CustomFieldOption, value string) bool {
	for _, opt := range options {
		if opt.Value == value {
			return true
		}
	}
	return false
}

// formatOptions returns a comma-separated list of option values.
func formatOptions(options []model.CustomFieldOption) string {
	values := make([]string, len(options))
	for i, opt := range options {
		values[i] = opt.Value
	}
	return strings.Join(values, ", ")
}

// parseSetValues parses a comma-separated string into a slice of values.
// Empty values are filtered out and values are trimmed.
func parseSetValues(value string) []string {
	if value == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// dedup removes duplicate strings, preserving order.
func dedup(vals []string) []string {
	seen := make(map[string]bool, len(vals))
	result := make([]string, 0, len(vals))
	for _, v := range vals {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// MissingWantedFieldOption describes a valid option for a missing wanted field.
type MissingWantedFieldOption struct {
	Value       string
	Description string
}

// MissingWantedField describes a wanted field that is missing from a card.
type MissingWantedField struct {
	FieldName   string                     // Name of the custom field
	FieldType   string                     // Type of the field (enum, enum-set, free-set, string, date)
	Description string                     // Field-level description
	Options     []MissingWantedFieldOption // For enum/enum-set, the valid options
}

// CheckWantedFieldsForProposal checks wanted fields for a proposed set of custom fields.
// Use this to validate input BEFORE creating/editing a card.
// For add operations, pass nil for existingFields.
// For edit operations, pass the current card's custom fields as existingFields.
func CheckWantedFieldsForProposal(existingFields map[string]any, proposedFields map[string]string, boardCfg *model.BoardConfig) []MissingWantedField {
	if boardCfg.CustomFields == nil {
		return nil
	}

	// Build merged fields
	merged := make(map[string]any)
	for k, v := range existingFields {
		merged[k] = v
	}

	// Apply proposed changes (with type conversion based on schema)
	for fieldName, value := range proposedFields {
		schema, exists := boardCfg.CustomFields[fieldName]
		if !exists {
			continue // Unknown field, skip (validation happens elsewhere)
		}

		switch schema.Type {
		case model.FieldTypeEnumSet, model.FieldTypeFreeSet:
			// Parse comma-separated values
			merged[fieldName] = parseSetValues(value)
		default:
			// String, enum, date - store as string
			merged[fieldName] = value
		}
	}

	tmpCard := &model.Card{CustomFields: merged}
	return CheckWantedFields(tmpCard, boardCfg)
}

// CheckWantedFields returns a list of wanted fields that are missing or empty on the card.
func CheckWantedFields(card *model.Card, boardCfg *model.BoardConfig) []MissingWantedField {
	var missing []MissingWantedField

	if boardCfg.CustomFields == nil {
		return missing
	}

	for name, schema := range boardCfg.CustomFields {
		if !schema.Wanted {
			continue
		}

		value, exists := card.CustomFields[name]
		if !exists || isEmpty(value, schema.Type) {
			mf := MissingWantedField{
				FieldName:   name,
				FieldType:   schema.Type,
				Description: schema.Description,
			}
			if schema.Type == model.FieldTypeEnum || schema.Type == model.FieldTypeEnumSet {
				mf.Options = make([]MissingWantedFieldOption, len(schema.Options))
				for i, opt := range schema.Options {
					mf.Options[i] = MissingWantedFieldOption{
						Value:       opt.Value,
						Description: opt.Description,
					}
				}
			}
			missing = append(missing, mf)
		}
	}

	return missing
}

// isEmpty checks if a custom field value is empty for its type.
func isEmpty(value any, fieldType string) bool {
	if value == nil {
		return true
	}

	switch fieldType {
	case model.FieldTypeString, model.FieldTypeEnum, model.FieldTypeDate:
		s, ok := value.(string)
		return !ok || s == ""
	case model.FieldTypeEnumSet, model.FieldTypeFreeSet:
		switch v := value.(type) {
		case []string:
			return len(v) == 0
		case []any:
			return len(v) == 0
		default:
			return true
		}
	default:
		return true
	}
}

// AddComment adds a new comment to a card.
func (s *CardService) AddComment(boardName, cardIDOrAlias, body, author string) (*model.Comment, error) {
	// Resolve card
	card, err := s.FindByIDOrAlias(boardName, cardIDOrAlias)
	if err != nil {
		return nil, err
	}

	// Create comment
	now := util.NowMillis()
	comment := model.Comment{
		ID:              id.Generate(id.Comment),
		Body:            body,
		Author:          author,
		CreatedAtMillis: now,
	}

	// Add to card's comments
	card.Comments = append(card.Comments, comment)

	// Save card
	if err := s.Update(boardName, card); err != nil {
		return nil, err
	}

	return &comment, nil
}

// EditComment updates an existing comment's body.
func (s *CardService) EditComment(boardName, commentID, body string) (*model.Comment, error) {
	// Find card containing this comment
	card, err := s.FindCommentCard(boardName, commentID)
	if err != nil {
		return nil, err
	}

	// Find and update the comment
	for i := range card.Comments {
		if card.Comments[i].ID == commentID {
			card.Comments[i].Body = body
			card.Comments[i].UpdatedAtMillis = util.NowMillis()

			// Save card
			if err := s.Update(boardName, card); err != nil {
				return nil, err
			}

			return &card.Comments[i], nil
		}
	}

	// Should not reach here if FindCommentCard worked
	return nil, kanerr.CommentNotFound(commentID)
}

// DeleteComment removes a comment from a card.
func (s *CardService) DeleteComment(boardName, commentID string) error {
	// Find card containing this comment
	card, err := s.FindCommentCard(boardName, commentID)
	if err != nil {
		return err
	}

	// Remove the comment
	for i, c := range card.Comments {
		if c.ID == commentID {
			card.Comments = append(card.Comments[:i], card.Comments[i+1:]...)
			return s.Update(boardName, card)
		}
	}

	// Should not reach here if FindCommentCard worked
	return kanerr.CommentNotFound(commentID)
}

// FindCommentCard finds the card containing a comment with the given ID.
func (s *CardService) FindCommentCard(boardName, commentID string) (*model.Card, error) {
	cards, err := s.cardStore.List(boardName)
	if err != nil {
		return nil, err
	}

	for _, card := range cards {
		for _, comment := range card.Comments {
			if comment.ID == commentID {
				return card, nil
			}
		}
	}

	return nil, kanerr.CommentNotFound(commentID)
}
