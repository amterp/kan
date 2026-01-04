package resolver

import (
	"testing"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// mockCardStore implements store.CardStore for testing.
type mockCardStore struct {
	cards map[string]map[string]*model.Card // board -> cardID -> card
}

func newMockCardStore() *mockCardStore {
	return &mockCardStore{
		cards: make(map[string]map[string]*model.Card),
	}
}

func (m *mockCardStore) addCard(boardName string, card *model.Card) {
	if m.cards[boardName] == nil {
		m.cards[boardName] = make(map[string]*model.Card)
	}
	m.cards[boardName][card.ID] = card
}

func (m *mockCardStore) Create(boardName string, card *model.Card) error {
	return nil
}

func (m *mockCardStore) Get(boardName, cardID string) (*model.Card, error) {
	if board, ok := m.cards[boardName]; ok {
		if card, ok := board[cardID]; ok {
			return card, nil
		}
	}
	return nil, kanerr.CardNotFound(cardID)
}

func (m *mockCardStore) Update(boardName string, card *model.Card) error {
	return nil
}

func (m *mockCardStore) Delete(boardName, cardID string) error {
	return nil
}

func (m *mockCardStore) List(boardName string) ([]*model.Card, error) {
	var cards []*model.Card
	if board, ok := m.cards[boardName]; ok {
		for _, card := range board {
			cards = append(cards, card)
		}
	}
	return cards, nil
}

func (m *mockCardStore) FindByAlias(boardName, alias string) (*model.Card, error) {
	if board, ok := m.cards[boardName]; ok {
		for _, card := range board {
			if card.Alias == alias {
				return card, nil
			}
		}
	}
	return nil, kanerr.CardNotFound(alias)
}

var _ store.CardStore = (*mockCardStore)(nil)

// ============================================================================
// CardResolver Tests
// ============================================================================

func TestCardResolver_Resolve_ByID(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{
		ID:    "abc123",
		Alias: "fix-bug",
		Title: "Fix Bug",
	})

	resolver := NewCardResolver(mockStore)

	card, err := resolver.Resolve("main", "abc123")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if card.ID != "abc123" {
		t.Errorf("Expected card ID 'abc123', got %q", card.ID)
	}
}

func TestCardResolver_Resolve_ByAlias(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{
		ID:    "abc123",
		Alias: "fix-bug",
		Title: "Fix Bug",
	})

	resolver := NewCardResolver(mockStore)

	// Resolve by alias (ID lookup will fail first)
	card, err := resolver.Resolve("main", "fix-bug")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if card.ID != "abc123" {
		t.Errorf("Expected card ID 'abc123', got %q", card.ID)
	}
}

func TestCardResolver_Resolve_IDTakesPrecedence(t *testing.T) {
	mockStore := newMockCardStore()
	// Create a card where ID happens to match another card's alias
	mockStore.addCard("main", &model.Card{
		ID:    "fix-bug", // ID that looks like an alias
		Alias: "actual-alias",
		Title: "Card 1",
	})
	mockStore.addCard("main", &model.Card{
		ID:    "xyz789",
		Alias: "other-alias",
		Title: "Card 2",
	})

	resolver := NewCardResolver(mockStore)

	// When we search for "fix-bug", it should find by ID (fast path)
	card, err := resolver.Resolve("main", "fix-bug")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if card.ID != "fix-bug" {
		t.Errorf("Expected card with ID 'fix-bug', got %q", card.ID)
	}
}

func TestCardResolver_Resolve_NotFound(t *testing.T) {
	mockStore := newMockCardStore()
	resolver := NewCardResolver(mockStore)

	_, err := resolver.Resolve("main", "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent card")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardResolver_Resolve_WrongBoard(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{
		ID:    "abc123",
		Alias: "fix-bug",
		Title: "Fix Bug",
	})

	resolver := NewCardResolver(mockStore)

	// Card exists in "main", but we're looking in "other"
	_, err := resolver.Resolve("other", "abc123")
	if err == nil {
		t.Fatal("Expected error for card in wrong board")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestCardResolver_Resolve_EmptyBoardName(t *testing.T) {
	mockStore := newMockCardStore()
	resolver := NewCardResolver(mockStore)

	_, err := resolver.Resolve("", "some-id")
	if err == nil {
		t.Fatal("Expected error for empty board name")
	}
}

func TestCardResolver_Resolve_EmptyIDOrAlias(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{
		ID:    "abc123",
		Alias: "fix-bug",
		Title: "Fix Bug",
	})

	resolver := NewCardResolver(mockStore)

	_, err := resolver.Resolve("main", "")
	if err == nil {
		t.Fatal("Expected error for empty idOrAlias")
	}
}
