package service

import (
	"fmt"
	"sort"
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

	// Placement within the target column. At most one should be set; when none
	// is set the card is appended to the end (the historical default).
	Position   *int   // explicit index (0 = top, -1 = end, negatives count from end)
	BeforeCard string // canonical ID of the card to insert before
	AfterCard  string // canonical ID of the card to insert after
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

	// Placement within a column. Any of these triggers a move (which may be an
	// in-place reorder when Column is nil). At most one should be set.
	Position   *int   // explicit index (0 = top, -1 = end, negatives count from end)
	BeforeCard string // canonical ID of the card to insert before
	AfterCard  string // canonical ID of the card to insert after
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

	allCards, err := s.cardStore.List(input.BoardName)
	if err != nil {
		return nil, nil, err
	}

	// Determine column: explicit wins, otherwise infer from any anchor, else the
	// board default. Also validates an anchor lives in the chosen column.
	column, err := resolveTargetColumn(allCards, input.Column, boardCfg.GetDefaultColumn(),
		input.BeforeCard, input.AfterCard)
	if err != nil {
		return nil, nil, err
	}
	if !boardCfg.HasColumn(column) {
		return nil, nil, kanerr.ColumnNotFound(column, input.BoardName)
	}

	// Check column limit by counting existing cards in the column
	col := boardCfg.GetColumn(column)
	colCards := cardsInColumn(allCards, column)
	if col.Limit > 0 && len(colCards) >= col.Limit {
		return nil, nil, kanerr.ColumnLimitExceeded(column, col.Limit)
	}

	// Compute position from the requested placement (defaults to end).
	idx, err := resolveInsertIndex(colCards, input.Position, input.BeforeCard, input.AfterCard)
	if err != nil {
		return nil, nil, err
	}
	position := computePosition(colCards, idx)

	// Generate ID and alias
	cardID := id.Generate(id.Card)
	alias, err := s.aliasService.GenerateAlias(input.BoardName, input.Title, "")
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
		Column:          column,
		Position:        position,
		History: []model.HistoryEntry{
			{Field: "column", Value: column, At: now},
		},
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

	// Execute pattern hooks if configured
	var hookResults []*HookResult
	if s.hookService != nil && len(boardCfg.PatternHooks) > 0 {
		matchingHooks := s.hookService.FindMatchingHooks(boardCfg.PatternHooks, input.Title)
		if len(matchingHooks) > 0 {
			hookResults = s.hookService.ExecuteHooks(matchingHooks, cardID, input.BoardName)

			// Re-fetch card after hooks to capture any modifications they made.
			// Hooks can modify cards via commands like `kan edit`, so we need
			// to return the card's state AFTER hook execution, not before.
			if updatedCard, err := s.cardStore.Get(input.BoardName, cardID); err == nil {
				card = updatedCard
			}
			// If re-fetch fails, we still return the original card (non-fatal)
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
// Cards are returned in column order (as defined in board config), sorted
// by position within each column.
func (s *CardService) List(boardName string, columnFilter string) ([]*model.Card, error) {
	return s.listSorted(boardName, columnFilter, "", false)
}

// ListSorted is like List, but within each column the cards are ordered by the
// given custom field instead of by their manual position. This is a
// non-destructive view sort—card positions on disk are left untouched. An empty
// sortField behaves exactly like List. See model.SortCardsByField for the
// ordering rules (enum option order, unset-last, etc.).
func (s *CardService) ListSorted(boardName, columnFilter, sortField string, descending bool) ([]*model.Card, error) {
	return s.listSorted(boardName, columnFilter, sortField, descending)
}

func (s *CardService) listSorted(boardName, columnFilter, sortField string, descending bool) ([]*model.Card, error) {
	cards, err := s.cardStore.List(boardName)
	if err != nil {
		return nil, err
	}

	// Get board config for column ordering
	boardCfg, err := s.boardStore.Get(boardName)
	if err != nil {
		// Without board config, return cards sorted by position only
		sort.Slice(cards, func(i, j int) bool {
			if cards[i].Column != cards[j].Column {
				return cards[i].Column < cards[j].Column
			}
			return cards[i].Position < cards[j].Position
		})
		return cards, nil
	}

	// Group cards by column
	grouped := make(map[string][]*model.Card)
	for _, card := range cards {
		grouped[card.Column] = append(grouped[card.Column], card)
	}

	// Order each group: by the requested custom field if one was given,
	// otherwise by manual position (the default).
	for col := range grouped {
		grp := grouped[col]
		if sortField != "" {
			model.SortCardsByField(grp, boardCfg, sortField, descending)
		} else {
			sort.Slice(grp, func(i, j int) bool {
				if grp[i].Position == grp[j].Position {
					return grp[i].ID < grp[j].ID // tiebreaker
				}
				return grp[i].Position < grp[j].Position
			})
		}
	}

	// Build result in column order (as defined in board config)
	var result []*model.Card
	validColumns := make(map[string]bool)
	for _, col := range boardCfg.Columns {
		validColumns[col.Name] = true
		if columnFilter != "" && col.Name != columnFilter {
			continue
		}
		result = append(result, grouped[col.Name]...)
	}

	// Append orphaned cards (column not in board config) if no filter
	if columnFilter == "" {
		for _, card := range cards {
			if !validColumns[card.Column] {
				result = append(result, card)
			}
		}
	}

	if result == nil {
		result = []*model.Card{}
	}
	return result, nil
}

// MoveCard moves a card to a different column at the bottom.
func (s *CardService) MoveCard(boardName, cardID, targetColumn string) error {
	return s.MoveCardWithPlacement(boardName, cardID, targetColumn, nil, "", "")
}

// MoveCardAt moves a card to a column at a specific index position.
// Position 0 = top of column, -1 = bottom.
func (s *CardService) MoveCardAt(boardName, cardID, targetColumn string, position int) error {
	return s.MoveCardWithPlacement(boardName, cardID, targetColumn, &position, "", "")
}

// MoveCardWithPlacement moves a card to a target column at a placement determined
// by exactly one of: an explicit index (position, non-nil), or an anchor card
// (beforeID/afterID, by canonical ID). When none is given, the card is appended
// to the end. An empty targetColumn is inferred from the anchor card's column
// when an anchor is given, otherwise the card stays in its current column (an
// in-place reorder).
func (s *CardService) MoveCardWithPlacement(boardName, cardID, targetColumn string,
	position *int, beforeID, afterID string) error {

	boardCfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return err
	}

	// Get the card to move
	card, err := s.cardStore.Get(boardName, cardID)
	if err != nil {
		return err
	}

	// A card cannot be placed relative to itself. The CLI guards this too, but
	// keep the invariant here so any direct caller gets a clear error rather than
	// a confusing "anchor not in column" downstream (the card is excluded from
	// the destination's card list).
	if firstNonEmpty(beforeID, afterID) == cardID {
		return fmt.Errorf("cannot move a card relative to itself")
	}

	// Load all cards to determine positions and check limits
	allCards, err := s.cardStore.List(boardName)
	if err != nil {
		return err
	}

	// Resolve the destination column: an explicit targetColumn wins, otherwise
	// infer from the anchor (if any) or fall back to the card's current column
	// (an in-place reorder). Also validates the anchor lives in the column.
	targetColumn, err = resolveTargetColumn(allCards, targetColumn, card.Column, beforeID, afterID)
	if err != nil {
		return err
	}

	if !boardCfg.HasColumn(targetColumn) {
		return kanerr.ColumnNotFound(targetColumn, boardName)
	}

	// Get sorted cards in target column (excluding the card being moved)
	colCards := cardsInColumnExcluding(allCards, targetColumn, cardID)

	// Check column limit for cross-column moves
	if card.Column != targetColumn {
		col := boardCfg.GetColumn(targetColumn)
		if col.Limit > 0 && len(colCards) >= col.Limit {
			return kanerr.ColumnLimitExceeded(targetColumn, col.Limit)
		}
	}

	idx, err := resolveInsertIndex(colCards, position, beforeID, afterID)
	if err != nil {
		return err
	}

	// Compute new position
	prevColumn := card.Column
	card.Column = targetColumn
	card.Position = computePosition(colCards, idx)
	card.UpdatedAtMillis = util.NowMillis()

	// Record the transition, but only on a genuine column change. Within-column
	// reorders flow through here too (targetColumn == prevColumn) and must not
	// append. Moving back to a previously-occupied column appends a fresh entry
	// by design - history is an append-only log, not a set.
	if prevColumn != targetColumn {
		card.History = append(card.History, model.HistoryEntry{
			Field: "column", Value: targetColumn, At: card.UpdatedAtMillis,
		})
	}

	return s.cardStore.Update(boardName, card)
}

// firstNonEmpty returns the first non-empty string, or "" if all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// resolveTargetColumn determines the destination column for a placement and
// validates that an anchor card (if any) lives there. An empty explicitColumn
// means "infer": from the anchor's column when an anchor is given, otherwise the
// supplied fallbackColumn (e.g. the card's current column, or the board default).
// This keeps column inference identical for both Add and MoveCardWithPlacement.
func resolveTargetColumn(allCards []*model.Card, explicitColumn, fallbackColumn, beforeID, afterID string) (string, error) {
	anchorID := firstNonEmpty(beforeID, afterID)
	if anchorID == "" {
		if explicitColumn != "" {
			return explicitColumn, nil
		}
		return fallbackColumn, nil
	}

	idx := indexOfCard(allCards, anchorID)
	if idx < 0 {
		return "", kanerr.CardNotFound(anchorID)
	}
	anchor := allCards[idx]
	if explicitColumn == "" {
		return anchor.Column, nil
	}
	if anchor.Column != explicitColumn {
		return "", fmt.Errorf("anchor card %q is in column %q, not %q", anchor.Alias, anchor.Column, explicitColumn)
	}
	return explicitColumn, nil
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
		alias, err := s.aliasService.GenerateAlias(boardName, newTitle, card.ID)
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

	// Handle column change and/or placement (an in-place reorder is possible
	// when only a placement is given without a column).
	if input.Column != nil || input.Position != nil || input.BeforeCard != "" || input.AfterCard != "" {
		targetColumn := "" // empty = infer (anchor's column, else current column)
		if input.Column != nil {
			if !boardCfg.HasColumn(*input.Column) {
				return nil, kanerr.ColumnNotFound(*input.Column, input.BoardName)
			}
			targetColumn = *input.Column
		}
		if err := s.MoveCardWithPlacement(input.BoardName, card.ID, targetColumn,
			input.Position, input.BeforeCard, input.AfterCard); err != nil {
			return nil, err
		}
		// Re-fetch card after move (column and position changed on disk)
		card, err = s.Get(input.BoardName, card.ID)
		if err != nil {
			return nil, err
		}
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
			if value == "" {
				delete(card.CustomFields, key)
			} else if !isValidOption(schema.Options, value) {
				return kanerr.InvalidField(key, fmt.Sprintf("must be one of: %s", formatOptions(schema.Options)))
			} else {
				card.CustomFields[key] = value
			}

		case model.FieldTypeEnumSet:
			// Parse comma-separated values, validate against options
			vals := parseSetValues(value)
			vals = dedup(vals)
			if len(vals) == 0 {
				delete(card.CustomFields, key)
			} else {
				if len(vals) > model.MaxSetItems {
					return kanerr.InvalidField(key, fmt.Sprintf("too many values (max %d)", model.MaxSetItems))
				}
				for _, v := range vals {
					if !isValidOption(schema.Options, v) {
						return kanerr.InvalidField(key, fmt.Sprintf("%q is not a valid option; must be one of: %s", v, formatOptions(schema.Options)))
					}
				}
				card.CustomFields[key] = vals
			}

		case model.FieldTypeFreeSet:
			// Parse comma-separated values, no option validation
			vals := parseSetValues(value)
			vals = dedup(vals)
			if len(vals) == 0 {
				delete(card.CustomFields, key)
			} else {
				if len(vals) > model.MaxSetItems {
					return kanerr.InvalidField(key, fmt.Sprintf("too many values (max %d)", model.MaxSetItems))
				}
				card.CustomFields[key] = vals
			}

		case model.FieldTypeBoolean:
			if value == "" {
				delete(card.CustomFields, key)
			} else {
				boolVal, err := parseBoolValue(value)
				if err != nil {
					return kanerr.InvalidField(key, err.Error())
				}
				card.CustomFields[key] = boolVal
			}

		case model.FieldTypeString, model.FieldTypeDate:
			if value == "" {
				delete(card.CustomFields, key)
			} else {
				card.CustomFields[key] = value
			}

		default:
			return kanerr.InvalidField(key, fmt.Sprintf("unknown field type %q", schema.Type))
		}
	}

	return nil
}

// Delete removes a card from the board.
func (s *CardService) Delete(boardName, cardID string) error {
	return s.cardStore.Delete(boardName, cardID)
}

// Restore re-creates a previously deleted card from a full snapshot.
// The card is written to disk as-is (preserving ID, alias, timestamps, etc.)
// and inserted into the specified column at the given position.
// If the original alias is taken, a new alias is generated.
func (s *CardService) Restore(boardName string, card *model.Card, column string, position int) error {
	// Validate card ID looks reasonable (prevent path traversal)
	if !id.IsValidID(card.ID) {
		return fmt.Errorf("invalid card ID: %s", card.ID)
	}

	// Validate board exists
	boardCfg, err := s.boardStore.Get(boardName)
	if err != nil {
		return err
	}

	// Validate column exists
	if !boardCfg.HasColumn(column) {
		return kanerr.ColumnNotFound(column, boardName)
	}

	// Check column limit by counting existing cards in the column
	col := boardCfg.GetColumn(column)
	allCards, err := s.cardStore.List(boardName)
	if err != nil {
		return err
	}
	colCards := cardsInColumn(allCards, column)
	if col.Limit > 0 && len(colCards) >= col.Limit {
		return kanerr.ColumnLimitExceeded(column, col.Limit)
	}

	// Check that no card with this ID already exists
	if _, err := s.cardStore.Get(boardName, card.ID); err == nil {
		return fmt.Errorf("card %s already exists", card.ID)
	}

	// Check alias availability - regenerate if taken by another card
	if !s.aliasService.IsAliasAvailable(boardName, card.Alias, card.ID) {
		alias, err := s.aliasService.GenerateAlias(boardName, card.Title, card.ID)
		if err != nil {
			return err
		}
		card.Alias = alias
		card.AliasExplicit = false
	}

	// Set column and position on the card itself
	card.Column = column
	card.Position = computePosition(colCards, position)

	// Write card file
	return s.cardStore.Create(boardName, card)
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

// parseBoolValue parses a string as a boolean value.
// Accepts true/false, yes/no, 1/0 (case-insensitive).
func parseBoolValue(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("must be one of: true, false, yes, no, 1, 0")
	}
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
	FieldType   string                     // Type of the field (enum, enum-set, free-set, string, date, boolean)
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
		case model.FieldTypeBoolean:
			boolVal, err := parseBoolValue(value)
			if err == nil {
				merged[fieldName] = boolVal
			} else {
				merged[fieldName] = value // let validation catch it
			}
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
	case model.FieldTypeBoolean:
		_, ok := value.(bool)
		return !ok
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

// cardsInColumn returns cards belonging to the given column, sorted by position.
func cardsInColumn(cards []*model.Card, column string) []*model.Card {
	var result []*model.Card
	for _, c := range cards {
		if c.Column == column {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Position == result[j].Position {
			return result[i].ID < result[j].ID
		}
		return result[i].Position < result[j].Position
	})
	return result
}

// cardsInColumnExcluding returns sorted cards in a column, excluding one card by ID.
func cardsInColumnExcluding(cards []*model.Card, column, excludeID string) []*model.Card {
	var result []*model.Card
	for _, c := range cards {
		if c.Column == column && c.ID != excludeID {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Position == result[j].Position {
			return result[i].ID < result[j].ID
		}
		return result[i].Position < result[j].Position
	})
	return result
}

// computePosition generates a fractional index position for inserting at the
// given index in a sorted slice of column cards.
// Index 0 = before first card; index >= n appends to the end. Negative indices
// count back from the end: -1 = end, -2 = before the last card, etc. A negative
// index that underflows past the top is clamped to the top.
func computePosition(sortedCards []*model.Card, index int) string {
	n := len(sortedCards)
	if n == 0 {
		return util.PositionBetween("", "")
	}
	if index < 0 {
		// Map -1 -> n (append), -2 -> n-1, ... so negatives count from the end.
		index = n + 1 + index
		if index < 0 {
			index = 0
		}
	}
	if index >= n {
		// Append to end
		return util.PositionAfter(sortedCards[n-1].Position)
	}
	if index == 0 {
		return util.PositionBefore(sortedCards[0].Position)
	}
	return util.PositionBetween(sortedCards[index-1].Position, sortedCards[index].Position)
}

// resolveInsertIndex returns the insertion index within colCards for a placement.
// colCards must be the destination column, sorted, excluding the card being moved.
// An explicit position (non-nil) takes precedence; otherwise beforeID/afterID
// anchor against a card already present in colCards (matched by canonical ID).
// Returns -1 (append to end) when no placement is given. Errors if an anchor card
// is not present in colCards.
func resolveInsertIndex(colCards []*model.Card, position *int, beforeID, afterID string) (int, error) {
	if position != nil {
		return *position, nil
	}
	if beforeID != "" {
		idx := indexOfCard(colCards, beforeID)
		if idx < 0 {
			return 0, fmt.Errorf("anchor card %q is not in the target column", beforeID)
		}
		return idx, nil
	}
	if afterID != "" {
		idx := indexOfCard(colCards, afterID)
		if idx < 0 {
			return 0, fmt.Errorf("anchor card %q is not in the target column", afterID)
		}
		return idx + 1, nil
	}
	return -1, nil
}

// indexOfCard returns the index of the card with the given ID in the slice, or -1.
func indexOfCard(cards []*model.Card, cardID string) int {
	for i, c := range cards {
		if c.ID == cardID {
			return i
		}
	}
	return -1
}
