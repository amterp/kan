package service

import (
	"testing"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// testCardStore implements store.CardStore for CardService testing.
// Unlike the alias_service mockCardStore, this tracks cards by ID.
type testCardStore struct {
	cards map[string]map[string]*model.Card // board -> cardID -> card
}

func newTestCardStore() *testCardStore {
	return &testCardStore{
		cards: make(map[string]map[string]*model.Card),
	}
}

func (m *testCardStore) Create(boardName string, card *model.Card) error {
	if m.cards[boardName] == nil {
		m.cards[boardName] = make(map[string]*model.Card)
	}
	m.cards[boardName][card.ID] = card
	return nil
}

func (m *testCardStore) Get(boardName, cardID string) (*model.Card, error) {
	if board, ok := m.cards[boardName]; ok {
		if card, ok := board[cardID]; ok {
			return card, nil
		}
	}
	return nil, kanerr.CardNotFound(cardID)
}

func (m *testCardStore) Update(boardName string, card *model.Card) error {
	if m.cards[boardName] == nil {
		return kanerr.CardNotFound(card.ID)
	}
	if _, ok := m.cards[boardName][card.ID]; !ok {
		return kanerr.CardNotFound(card.ID)
	}
	m.cards[boardName][card.ID] = card
	return nil
}

func (m *testCardStore) Delete(boardName, cardID string) error {
	if board, ok := m.cards[boardName]; ok {
		if _, ok := board[cardID]; ok {
			delete(board, cardID)
			return nil
		}
	}
	return kanerr.CardNotFound(cardID)
}

func (m *testCardStore) List(boardName string) ([]*model.Card, error) {
	var cards []*model.Card
	if board, ok := m.cards[boardName]; ok {
		for _, card := range board {
			cards = append(cards, card)
		}
	}
	return cards, nil
}

func (m *testCardStore) FindByAlias(boardName, alias string) (*model.Card, error) {
	if board, ok := m.cards[boardName]; ok {
		for _, card := range board {
			if card.Alias == alias {
				return card, nil
			}
		}
	}
	return nil, kanerr.CardNotFound(alias)
}

var _ store.CardStore = (*testCardStore)(nil)

// testBoardStore implements store.BoardStore for testing.
type testBoardStore struct {
	boards map[string]*model.BoardConfig
}

func newTestBoardStore() *testBoardStore {
	return &testBoardStore{
		boards: make(map[string]*model.BoardConfig),
	}
}

func (m *testBoardStore) addBoard(cfg *model.BoardConfig) {
	m.boards[cfg.Name] = cfg
}

func (m *testBoardStore) Create(config *model.BoardConfig) error {
	if _, ok := m.boards[config.Name]; ok {
		return kanerr.BoardAlreadyExists(config.Name)
	}
	m.boards[config.Name] = config
	return nil
}

func (m *testBoardStore) Get(boardName string) (*model.BoardConfig, error) {
	if cfg, ok := m.boards[boardName]; ok {
		return cfg, nil
	}
	return nil, kanerr.BoardNotFound(boardName)
}

func (m *testBoardStore) Update(config *model.BoardConfig) error {
	if _, ok := m.boards[config.Name]; !ok {
		return kanerr.BoardNotFound(config.Name)
	}
	m.boards[config.Name] = config
	return nil
}

func (m *testBoardStore) List() ([]string, error) {
	var names []string
	for name := range m.boards {
		names = append(names, name)
	}
	return names, nil
}

func (m *testBoardStore) Exists(boardName string) bool {
	_, ok := m.boards[boardName]
	return ok
}

var _ store.BoardStore = (*testBoardStore)(nil)

// Helper to create a basic board config for testing
func testBoardConfig(name string) *model.BoardConfig {
	return &model.BoardConfig{
		ID:            "test-board-id",
		Name:          name,
		DefaultColumn: "backlog",
		Columns: []model.Column{
			{Name: "backlog", Color: "#6b7280"},
			{Name: "in-progress", Color: "#f59e0b"},
			{Name: "done", Color: "#10b981"},
		},
		Labels: []model.Label{
			{Name: "bug", Color: "#ef4444"},
			{Name: "feature", Color: "#3b82f6"},
		},
	}
}

// Helper to set up CardService with test stores
func setupCardService() (*CardService, *testCardStore, *testBoardStore) {
	cardStore := newTestCardStore()
	boardStore := newTestBoardStore()
	aliasService := NewAliasService(cardStore)
	service := NewCardService(cardStore, boardStore, aliasService)
	return service, cardStore, boardStore
}

// ============================================================================
// Add() Tests
// ============================================================================

func TestCardService_Add_Basic(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, err := service.Add(AddCardInput{
		BoardName: "main",
		Title:     "Fix login bug",
		Column:    "backlog",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if card.ID == "" {
		t.Error("Card ID should not be empty")
	}
	if card.Alias != "fix-login-bug" {
		t.Errorf("Expected alias 'fix-login-bug', got %q", card.Alias)
	}
	if card.Title != "Fix login bug" {
		t.Errorf("Expected title 'Fix login bug', got %q", card.Title)
	}
	if card.Column != "backlog" {
		t.Errorf("Expected column 'backlog', got %q", card.Column)
	}
	if card.AliasExplicit {
		t.Error("AliasExplicit should be false for auto-generated alias")
	}
	if card.CreatedAtMillis == 0 {
		t.Error("CreatedAtMillis should be set")
	}
	if card.UpdatedAtMillis == 0 {
		t.Error("UpdatedAtMillis should be set")
	}
}

func TestCardService_Add_WithLabels(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, err := service.Add(AddCardInput{
		BoardName: "main",
		Title:     "New feature",
		Column:    "backlog",
		Labels:    []string{"bug", "feature"},
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if len(card.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(card.Labels))
	}
}

func TestCardService_Add_DefaultColumn(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, err := service.Add(AddCardInput{
		BoardName: "main",
		Title:     "No column specified",
		Column:    "", // Should use default
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if card.Column != "backlog" {
		t.Errorf("Expected default column 'backlog', got %q", card.Column)
	}
}

func TestCardService_Add_InvalidColumn(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	_, err := service.Add(AddCardInput{
		BoardName: "main",
		Title:     "Bad column",
		Column:    "NonExistent",
	})
	if err == nil {
		t.Fatal("Expected error for invalid column")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardService_Add_InvalidLabel(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	_, err := service.Add(AddCardInput{
		BoardName: "main",
		Title:     "Bad label",
		Column:    "backlog",
		Labels:    []string{"bug", "nonexistent-label"},
	})
	if err == nil {
		t.Fatal("Expected error for invalid label")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardService_Add_BoardNotFound(t *testing.T) {
	service, _, _ := setupCardService()
	// No board added

	_, err := service.Add(AddCardInput{
		BoardName: "nonexistent",
		Title:     "Test",
		Column:    "backlog",
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent board")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardService_Add_UpdatesBoardConfig(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, err := service.Add(AddCardInput{
		BoardName: "main",
		Title:     "Test card",
		Column:    "in-progress",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Verify card was added to board config's column
	cfg, _ := boardStore.Get("main")
	found := false
	for _, col := range cfg.Columns {
		if col.Name == "in-progress" {
			for _, id := range col.CardIDs {
				if id == card.ID {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("Card ID should be in board config's column CardIDs")
	}
}

// ============================================================================
// Get() Tests
// ============================================================================

func TestCardService_Get_Found(t *testing.T) {
	service, cardStore, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	// Create a card first
	created, _ := service.Add(AddCardInput{
		BoardName: "main",
		Title:     "Test",
		Column:    "backlog",
	})

	card, err := service.Get("main", created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if card.ID != created.ID {
		t.Errorf("Expected card ID %q, got %q", created.ID, card.ID)
	}

	_ = cardStore // silence unused warning
}

func TestCardService_Get_NotFound(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	_, err := service.Get("main", "nonexistent-id")
	if err == nil {
		t.Fatal("Expected error for nonexistent card")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

// ============================================================================
// Update() Tests
// ============================================================================

func TestCardService_Update_ModifiesTimestamp(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{
		BoardName: "main",
		Title:     "Test",
		Column:    "backlog",
	})
	originalCreated := card.CreatedAtMillis

	// Modify and update
	card.Description = "Updated description"
	if err := service.Update("main", card); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Fetch again
	updated, _ := service.Get("main", card.ID)

	if updated.CreatedAtMillis != originalCreated {
		t.Error("CreatedAtMillis should not change on update")
	}
	// Note: UpdatedAtMillis is set by Update(), so it should be >= original
	// (may be same millisecond in fast tests)
	if updated.UpdatedAtMillis < originalCreated {
		t.Error("UpdatedAtMillis should be set")
	}
	if updated.Description != "Updated description" {
		t.Error("Description should be updated")
	}
}

// ============================================================================
// List() Tests
// ============================================================================

func TestCardService_List_Empty(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	cards, err := service.List("main", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if cards == nil {
		t.Error("List should return empty slice, not nil")
	}
	if len(cards) != 0 {
		t.Errorf("Expected 0 cards, got %d", len(cards))
	}
}

func TestCardService_List_ReturnsAllCards(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	// Add cards to different columns
	service.Add(AddCardInput{BoardName: "main", Title: "Card 1", Column: "backlog"})
	service.Add(AddCardInput{BoardName: "main", Title: "Card 2", Column: "in-progress"})
	service.Add(AddCardInput{BoardName: "main", Title: "Card 3", Column: "done"})

	cards, err := service.List("main", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(cards) != 3 {
		t.Errorf("Expected 3 cards, got %d", len(cards))
	}
}

func TestCardService_List_WithColumnFilter(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	service.Add(AddCardInput{BoardName: "main", Title: "backlog 1", Column: "backlog"})
	service.Add(AddCardInput{BoardName: "main", Title: "backlog 2", Column: "backlog"})
	service.Add(AddCardInput{BoardName: "main", Title: "in-progress 1", Column: "in-progress"})

	cards, err := service.List("main", "backlog")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(cards) != 2 {
		t.Errorf("Expected 2 cards in backlog, got %d", len(cards))
	}
	for _, card := range cards {
		if card.Column != "backlog" {
			t.Errorf("Expected column 'backlog', got %q", card.Column)
		}
	}
}

func TestCardService_List_OrderedByBoardConfig(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	// Add cards - they should be returned in column order (backlog, in-progress, done)
	card1, _ := service.Add(AddCardInput{BoardName: "main", Title: "done card", Column: "done"})
	card2, _ := service.Add(AddCardInput{BoardName: "main", Title: "backlog card", Column: "backlog"})
	card3, _ := service.Add(AddCardInput{BoardName: "main", Title: "in-progress card", Column: "in-progress"})

	cards, err := service.List("main", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("Expected 3 cards, got %d", len(cards))
	}

	// Verify order: backlog first, then in-progress, then done
	if cards[0].ID != card2.ID {
		t.Error("First card should be from backlog column")
	}
	if cards[1].ID != card3.ID {
		t.Error("Second card should be from in-progress column")
	}
	if cards[2].ID != card1.ID {
		t.Error("Third card should be from done column")
	}
}

// ============================================================================
// MoveCard() / MoveCardAt() Tests
// ============================================================================

func TestCardService_MoveCard_ToEnd(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	if err := service.MoveCard("main", card.ID, "in-progress"); err != nil {
		t.Fatalf("MoveCard failed: %v", err)
	}

	// Verify board config is updated (column membership is stored in board config only)
	cfg, _ := boardStore.Get("main")
	found := false
	for _, col := range cfg.Columns {
		if col.Name == "in-progress" {
			for _, id := range col.CardIDs {
				if id == card.ID {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("Card should be in in-progress column's CardIDs")
	}

	// Verify card is removed from old column
	for _, col := range cfg.Columns {
		if col.Name == "backlog" {
			for _, id := range col.CardIDs {
				if id == card.ID {
					t.Error("Card should be removed from backlog column's CardIDs")
				}
			}
		}
	}
}

func TestCardService_MoveCardAt_Position(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	// Add two cards to in-progress
	card1, _ := service.Add(AddCardInput{BoardName: "main", Title: "First", Column: "in-progress"})
	card2, _ := service.Add(AddCardInput{BoardName: "main", Title: "Second", Column: "in-progress"})

	// Add a third card to backlog, then move it to position 0 in in-progress
	card3, _ := service.Add(AddCardInput{BoardName: "main", Title: "Third", Column: "backlog"})

	if err := service.MoveCardAt("main", card3.ID, "in-progress", 0); err != nil {
		t.Fatalf("MoveCardAt failed: %v", err)
	}

	// Verify order in board config
	cfg, _ := boardStore.Get("main")
	var inProgressIDs []string
	for _, col := range cfg.Columns {
		if col.Name == "in-progress" {
			inProgressIDs = col.CardIDs
			break
		}
	}

	if len(inProgressIDs) != 3 {
		t.Fatalf("Expected 3 cards in in-progress, got %d", len(inProgressIDs))
	}
	if inProgressIDs[0] != card3.ID {
		t.Error("Card3 should be at position 0")
	}
	if inProgressIDs[1] != card1.ID {
		t.Error("Card1 should be at position 1")
	}
	if inProgressIDs[2] != card2.ID {
		t.Error("Card2 should be at position 2")
	}
}

func TestCardService_MoveCard_InvalidColumn(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	err := service.MoveCard("main", card.ID, "NonExistent")
	if err == nil {
		t.Fatal("Expected error for invalid column")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardService_MoveCard_CardNotFound(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	err := service.MoveCard("main", "nonexistent-id", "in-progress")
	if err == nil {
		t.Fatal("Expected error for nonexistent card")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardService_MoveCard_SameColumn(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	// Move to same column - should still work
	if err := service.MoveCard("main", card.ID, "backlog"); err != nil {
		t.Fatalf("MoveCard to same column failed: %v", err)
	}

	// Verify card still in backlog (column membership in board config)
	cfg, _ := boardStore.Get("main")
	found := false
	for _, col := range cfg.Columns {
		if col.Name == "backlog" {
			for _, id := range col.CardIDs {
				if id == card.ID {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("Card should still be in backlog column's CardIDs")
	}
}

// ============================================================================
// FindByIDOrAlias() Tests
// ============================================================================

func TestCardService_FindByIDOrAlias_ByID(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	created, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	card, err := service.FindByIDOrAlias("main", created.ID)
	if err != nil {
		t.Fatalf("FindByIDOrAlias failed: %v", err)
	}
	if card.ID != created.ID {
		t.Errorf("Expected card ID %q, got %q", created.ID, card.ID)
	}
}

func TestCardService_FindByIDOrAlias_ByAlias(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	created, _ := service.Add(AddCardInput{BoardName: "main", Title: "Fix login bug", Column: "backlog"})

	card, err := service.FindByIDOrAlias("main", "fix-login-bug")
	if err != nil {
		t.Fatalf("FindByIDOrAlias failed: %v", err)
	}
	if card.ID != created.ID {
		t.Errorf("Expected card ID %q, got %q", created.ID, card.ID)
	}
}

func TestCardService_FindByIDOrAlias_NotFound(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	_, err := service.FindByIDOrAlias("main", "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent card")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

// ============================================================================
// UpdateTitle() Tests
// ============================================================================

func TestCardService_UpdateTitle_RegeneratesAlias(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Original title", Column: "backlog"})
	originalAlias := card.Alias

	if err := service.UpdateTitle("main", card, "New title"); err != nil {
		t.Fatalf("UpdateTitle failed: %v", err)
	}

	updated, _ := service.Get("main", card.ID)
	if updated.Title != "New title" {
		t.Errorf("Expected title 'New title', got %q", updated.Title)
	}
	if updated.Alias == originalAlias {
		t.Error("Alias should be regenerated when AliasExplicit is false")
	}
	if updated.Alias != "new-title" {
		t.Errorf("Expected alias 'new-title', got %q", updated.Alias)
	}
}

func TestCardService_UpdateTitle_PreservesExplicitAlias(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Original title", Column: "backlog"})

	// Mark alias as explicit
	card.AliasExplicit = true
	card.Alias = "my-custom-alias"
	service.Update("main", card)

	if err := service.UpdateTitle("main", card, "New title"); err != nil {
		t.Fatalf("UpdateTitle failed: %v", err)
	}

	updated, _ := service.Get("main", card.ID)
	if updated.Title != "New title" {
		t.Errorf("Expected title 'New title', got %q", updated.Title)
	}
	if updated.Alias != "my-custom-alias" {
		t.Errorf("Expected alias 'my-custom-alias' (preserved), got %q", updated.Alias)
	}
}

// ============================================================================
// Delete() Tests
// ============================================================================

func TestCardService_Delete_RemovesFromBothStores(t *testing.T) {
	service, cardStore, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "To delete", Column: "backlog"})

	if err := service.Delete("main", card.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify removed from card store
	_, err := cardStore.Get("main", card.ID)
	if !kanerr.IsNotFound(err) {
		t.Error("Card should be removed from card store")
	}

	// Verify removed from board config
	cfg, _ := boardStore.Get("main")
	for _, col := range cfg.Columns {
		for _, id := range col.CardIDs {
			if id == card.ID {
				t.Error("Card ID should be removed from board config")
			}
		}
	}
}

func TestCardService_Delete_CardNotFound(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	err := service.Delete("main", "nonexistent-id")
	if err == nil {
		t.Fatal("Expected error for nonexistent card")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

// ============================================================================
// Edit() Tests
// ============================================================================

func TestCardService_Edit_Title(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Original", Column: "backlog"})

	newTitle := "Updated Title"
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Title:         &newTitle,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %q", updated.Title)
	}
	// Alias should be regenerated since it wasn't explicit
	if updated.Alias != "updated-title" {
		t.Errorf("Expected alias 'updated-title', got %q", updated.Alias)
	}
}

func TestCardService_Edit_Title_EmptyError(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Original", Column: "backlog"})

	emptyTitle := ""
	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Title:         &emptyTitle,
	})
	if err == nil {
		t.Fatal("Expected error for empty title")
	}
	if !kanerr.IsValidationError(err) {
		t.Errorf("Expected validation error, got %v", err)
	}
}

func TestCardService_Edit_Description(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	newDesc := "New description"
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Description:   &newDesc,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.Description != "New description" {
		t.Errorf("Expected description 'New description', got %q", updated.Description)
	}
}

func TestCardService_Edit_Column(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	newColumn := "in-progress"
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Column:        &newColumn,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.Column != "in-progress" {
		t.Errorf("Expected column 'in-progress', got %q", updated.Column)
	}

	// Verify board config was updated
	cfg, _ := boardStore.Get("main")
	found := false
	for _, col := range cfg.Columns {
		if col.Name == "in-progress" {
			for _, id := range col.CardIDs {
				if id == card.ID {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("Card should be in in-progress column's CardIDs")
	}
}

func TestCardService_Edit_Column_Invalid(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	badColumn := "nonexistent"
	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Column:        &badColumn,
	})
	if err == nil {
		t.Fatal("Expected error for invalid column")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardService_Edit_Labels(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog", Labels: []string{"bug"}})

	newLabels := []string{"feature"}
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Labels:        &newLabels,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if len(updated.Labels) != 1 || updated.Labels[0] != "feature" {
		t.Errorf("Expected labels [feature], got %v", updated.Labels)
	}
}

func TestCardService_Edit_Labels_Clear(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog", Labels: []string{"bug", "feature"}})

	emptyLabels := []string{""}
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Labels:        &emptyLabels,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if len(updated.Labels) != 0 {
		t.Errorf("Expected empty labels, got %v", updated.Labels)
	}
}

func TestCardService_Edit_Labels_Invalid(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	badLabels := []string{"nonexistent-label"}
	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Labels:        &badLabels,
	})
	if err == nil {
		t.Fatal("Expected error for invalid label")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardService_Edit_Parent(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	parentCard, _ := service.Add(AddCardInput{BoardName: "main", Title: "Parent", Column: "backlog"})
	childCard, _ := service.Add(AddCardInput{BoardName: "main", Title: "Child", Column: "backlog"})

	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: childCard.ID,
		Parent:        &parentCard.ID,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.Parent != parentCard.ID {
		t.Errorf("Expected parent %q, got %q", parentCard.ID, updated.Parent)
	}
}

func TestCardService_Edit_Parent_Clear(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	parentCard, _ := service.Add(AddCardInput{BoardName: "main", Title: "Parent", Column: "backlog"})
	childCard, _ := service.Add(AddCardInput{BoardName: "main", Title: "Child", Column: "backlog", Parent: parentCard.ID})

	emptyParent := ""
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: childCard.ID,
		Parent:        &emptyParent,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.Parent != "" {
		t.Errorf("Expected empty parent, got %q", updated.Parent)
	}
}

func TestCardService_Edit_Parent_NotFound(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	badParent := "nonexistent-parent"
	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Parent:        &badParent,
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent parent")
	}
}

func TestCardService_Edit_Alias(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	newAlias := "my-custom-alias"
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Alias:         &newAlias,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.Alias != "my-custom-alias" {
		t.Errorf("Expected alias 'my-custom-alias', got %q", updated.Alias)
	}
	if !updated.AliasExplicit {
		t.Error("AliasExplicit should be true after setting explicit alias")
	}
}

func TestCardService_Edit_Alias_Empty(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	emptyAlias := ""
	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Alias:         &emptyAlias,
	})
	if err == nil {
		t.Fatal("Expected error for empty alias")
	}
	if !kanerr.IsValidationError(err) {
		t.Errorf("Expected validation error, got %v", err)
	}
}

func TestCardService_Edit_Alias_Collision(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	service.Add(AddCardInput{BoardName: "main", Title: "First card", Column: "backlog"})
	card2, _ := service.Add(AddCardInput{BoardName: "main", Title: "Second card", Column: "backlog"})

	// Try to set card2's alias to card1's alias
	conflictingAlias := "first-card"
	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card2.ID,
		Alias:         &conflictingAlias,
	})
	if err == nil {
		t.Fatal("Expected error for alias collision")
	}
	if !kanerr.IsValidationError(err) {
		t.Errorf("Expected validation error, got %v", err)
	}
}

func TestCardService_Edit_MultipleFields(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Original", Column: "backlog"})

	newTitle := "Updated"
	newDesc := "New description"
	newColumn := "in-progress"
	newLabels := []string{"bug", "feature"}

	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		Title:         &newTitle,
		Description:   &newDesc,
		Column:        &newColumn,
		Labels:        &newLabels,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.Title != "Updated" {
		t.Errorf("Expected title 'Updated', got %q", updated.Title)
	}
	if updated.Description != "New description" {
		t.Errorf("Expected description 'New description', got %q", updated.Description)
	}
	if updated.Column != "in-progress" {
		t.Errorf("Expected column 'in-progress', got %q", updated.Column)
	}
	if len(updated.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(updated.Labels))
	}
}

func TestCardService_Edit_NilFieldsNoChange(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{
		BoardName:   "main",
		Title:       "Original Title",
		Description: "Original Desc",
		Column:      "backlog",
		Labels:      []string{"bug"},
	})

	// Edit with all nil fields - should change nothing
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.Title != "Original Title" {
		t.Errorf("Title should be unchanged, got %q", updated.Title)
	}
	if updated.Description != "Original Desc" {
		t.Errorf("Description should be unchanged, got %q", updated.Description)
	}
	if len(updated.Labels) != 1 || updated.Labels[0] != "bug" {
		t.Errorf("Labels should be unchanged, got %v", updated.Labels)
	}
}

func TestCardService_Edit_ByAlias(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Fix login bug", Column: "backlog"})

	newDesc := "Updated via alias"
	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: "fix-login-bug", // Use alias instead of ID
		Description:   &newDesc,
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.ID != card.ID {
		t.Errorf("Expected card ID %q, got %q", card.ID, updated.ID)
	}
	if updated.Description != "Updated via alias" {
		t.Errorf("Expected description 'Updated via alias', got %q", updated.Description)
	}
}

func TestCardService_Edit_CardNotFound(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	newTitle := "Updated"
	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: "nonexistent",
		Title:         &newTitle,
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent card")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

// Helper to create a board config with custom fields for testing
func testBoardConfigWithCustomFields(name string) *model.BoardConfig {
	cfg := testBoardConfig(name)
	cfg.CustomFields = map[string]model.CustomFieldSchema{
		"priority": {Type: "enum", Values: []string{"low", "medium", "high"}},
		"estimate": {Type: "string"},
	}
	return cfg
}

func TestCardService_Edit_CustomFields(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfigWithCustomFields("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	updated, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		CustomFields:  map[string]string{"priority": "high", "estimate": "3"},
	})
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	if updated.CustomFields["priority"] != "high" {
		t.Errorf("Expected priority 'high', got %v", updated.CustomFields["priority"])
	}
	if updated.CustomFields["estimate"] != "3" {
		t.Errorf("Expected estimate '3', got %v", updated.CustomFields["estimate"])
	}
}

func TestCardService_Edit_CustomFields_InvalidEnum(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfigWithCustomFields("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		CustomFields:  map[string]string{"priority": "invalid-value"},
	})
	if err == nil {
		t.Fatal("Expected error for invalid enum value")
	}
	if !kanerr.IsValidationError(err) {
		t.Errorf("Expected validation error, got %v", err)
	}
}

func TestCardService_Edit_CustomFields_UndefinedField(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfigWithCustomFields("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		CustomFields:  map[string]string{"undefined_field": "value"},
	})
	if err == nil {
		t.Fatal("Expected error for undefined custom field")
	}
	if !kanerr.IsValidationError(err) {
		t.Errorf("Expected validation error, got %v", err)
	}
}

func TestCardService_Edit_CustomFields_ReservedPrefix(t *testing.T) {
	service, _, boardStore := setupCardService()
	// Add a board with a field that has reserved prefix (shouldn't happen in practice, but tests validation)
	cfg := testBoardConfig("main")
	cfg.CustomFields = map[string]model.CustomFieldSchema{
		"valid_field": {Type: "string"},
	}
	boardStore.addBoard(cfg)

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "backlog"})

	// Try to set a field with reserved prefix
	_, err := service.Edit(EditCardInput{
		BoardName:     "main",
		CardIDOrAlias: card.ID,
		CustomFields:  map[string]string{"_reserved": "value"},
	})
	if err == nil {
		t.Fatal("Expected error for reserved prefix field")
	}
}
