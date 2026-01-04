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
		DefaultColumn: "Backlog",
		Columns: []model.Column{
			{Name: "Backlog", Color: "#6b7280"},
			{Name: "In Progress", Color: "#f59e0b"},
			{Name: "Done", Color: "#10b981"},
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
		Column:    "Backlog",
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
	if card.Column != "Backlog" {
		t.Errorf("Expected column 'Backlog', got %q", card.Column)
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
		Column:    "Backlog",
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

	if card.Column != "Backlog" {
		t.Errorf("Expected default column 'Backlog', got %q", card.Column)
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
		Column:    "Backlog",
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
		Column:    "Backlog",
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
		Column:    "In Progress",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Verify card was added to board config's column
	cfg, _ := boardStore.Get("main")
	found := false
	for _, col := range cfg.Columns {
		if col.Name == "In Progress" {
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
		Column:    "Backlog",
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
		Column:    "Backlog",
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
	service.Add(AddCardInput{BoardName: "main", Title: "Card 1", Column: "Backlog"})
	service.Add(AddCardInput{BoardName: "main", Title: "Card 2", Column: "In Progress"})
	service.Add(AddCardInput{BoardName: "main", Title: "Card 3", Column: "Done"})

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

	service.Add(AddCardInput{BoardName: "main", Title: "Backlog 1", Column: "Backlog"})
	service.Add(AddCardInput{BoardName: "main", Title: "Backlog 2", Column: "Backlog"})
	service.Add(AddCardInput{BoardName: "main", Title: "In Progress 1", Column: "In Progress"})

	cards, err := service.List("main", "Backlog")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(cards) != 2 {
		t.Errorf("Expected 2 cards in Backlog, got %d", len(cards))
	}
	for _, card := range cards {
		if card.Column != "Backlog" {
			t.Errorf("Expected column 'Backlog', got %q", card.Column)
		}
	}
}

func TestCardService_List_OrderedByBoardConfig(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	// Add cards - they should be returned in column order (Backlog, In Progress, Done)
	card1, _ := service.Add(AddCardInput{BoardName: "main", Title: "Done card", Column: "Done"})
	card2, _ := service.Add(AddCardInput{BoardName: "main", Title: "Backlog card", Column: "Backlog"})
	card3, _ := service.Add(AddCardInput{BoardName: "main", Title: "In Progress card", Column: "In Progress"})

	cards, err := service.List("main", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("Expected 3 cards, got %d", len(cards))
	}

	// Verify order: Backlog first, then In Progress, then Done
	if cards[0].ID != card2.ID {
		t.Error("First card should be from Backlog column")
	}
	if cards[1].ID != card3.ID {
		t.Error("Second card should be from In Progress column")
	}
	if cards[2].ID != card1.ID {
		t.Error("Third card should be from Done column")
	}
}

// ============================================================================
// MoveCard() / MoveCardAt() Tests
// ============================================================================

func TestCardService_MoveCard_ToEnd(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "Backlog"})

	if err := service.MoveCard("main", card.ID, "In Progress"); err != nil {
		t.Fatalf("MoveCard failed: %v", err)
	}

	// Verify card's column is updated
	moved, _ := service.Get("main", card.ID)
	if moved.Column != "In Progress" {
		t.Errorf("Expected column 'In Progress', got %q", moved.Column)
	}

	// Verify board config is updated
	cfg, _ := boardStore.Get("main")
	found := false
	for _, col := range cfg.Columns {
		if col.Name == "In Progress" {
			for _, id := range col.CardIDs {
				if id == card.ID {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("Card should be in In Progress column's CardIDs")
	}
}

func TestCardService_MoveCardAt_Position(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	// Add two cards to In Progress
	card1, _ := service.Add(AddCardInput{BoardName: "main", Title: "First", Column: "In Progress"})
	card2, _ := service.Add(AddCardInput{BoardName: "main", Title: "Second", Column: "In Progress"})

	// Add a third card to Backlog, then move it to position 0 in In Progress
	card3, _ := service.Add(AddCardInput{BoardName: "main", Title: "Third", Column: "Backlog"})

	if err := service.MoveCardAt("main", card3.ID, "In Progress", 0); err != nil {
		t.Fatalf("MoveCardAt failed: %v", err)
	}

	// Verify order in board config
	cfg, _ := boardStore.Get("main")
	var inProgressIDs []string
	for _, col := range cfg.Columns {
		if col.Name == "In Progress" {
			inProgressIDs = col.CardIDs
			break
		}
	}

	if len(inProgressIDs) != 3 {
		t.Fatalf("Expected 3 cards in In Progress, got %d", len(inProgressIDs))
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

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "Backlog"})

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

	err := service.MoveCard("main", "nonexistent-id", "In Progress")
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

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "Backlog"})
	originalUpdated := card.UpdatedAtMillis

	// Move to same column - should still work (updates timestamp)
	if err := service.MoveCard("main", card.ID, "Backlog"); err != nil {
		t.Fatalf("MoveCard to same column failed: %v", err)
	}

	// Verify card still in Backlog
	moved, _ := service.Get("main", card.ID)
	if moved.Column != "Backlog" {
		t.Errorf("Expected column 'Backlog', got %q", moved.Column)
	}
	// Timestamp should be updated
	if moved.UpdatedAtMillis < originalUpdated {
		t.Error("UpdatedAtMillis should be updated even for same-column move")
	}
}

// ============================================================================
// FindByIDOrAlias() Tests
// ============================================================================

func TestCardService_FindByIDOrAlias_ByID(t *testing.T) {
	service, _, boardStore := setupCardService()
	boardStore.addBoard(testBoardConfig("main"))

	created, _ := service.Add(AddCardInput{BoardName: "main", Title: "Test", Column: "Backlog"})

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

	created, _ := service.Add(AddCardInput{BoardName: "main", Title: "Fix login bug", Column: "Backlog"})

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

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Original title", Column: "Backlog"})
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

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "Original title", Column: "Backlog"})

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

	card, _ := service.Add(AddCardInput{BoardName: "main", Title: "To delete", Column: "Backlog"})

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
