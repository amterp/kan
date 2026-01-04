package service

import (
	"testing"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

// mockCardStore implements store.CardStore for testing
type mockCardStore struct {
	cards map[string]map[string]*model.Card // board -> alias -> card
}

func newMockCardStore() *mockCardStore {
	return &mockCardStore{
		cards: make(map[string]map[string]*model.Card),
	}
}

func (m *mockCardStore) addCard(board string, card *model.Card) {
	if m.cards[board] == nil {
		m.cards[board] = make(map[string]*model.Card)
	}
	m.cards[board][card.Alias] = card
}

func (m *mockCardStore) Create(boardName string, card *model.Card) error {
	return nil
}

func (m *mockCardStore) Get(boardName, cardID string) (*model.Card, error) {
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
	for _, card := range m.cards[boardName] {
		cards = append(cards, card)
	}
	return cards, nil
}

func (m *mockCardStore) FindByAlias(boardName, alias string) (*model.Card, error) {
	if board, ok := m.cards[boardName]; ok {
		if card, ok := board[alias]; ok {
			return card, nil
		}
	}
	return nil, kanerr.CardNotFound(alias)
}

// Ensure mockCardStore implements the interface
var _ store.CardStore = (*mockCardStore)(nil)

func TestAliasService_GenerateAlias_Basic(t *testing.T) {
	mockStore := newMockCardStore()
	service := NewAliasService(mockStore)

	alias, err := service.GenerateAlias("main", "Fix login bug")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	if alias != "fix-login-bug" {
		t.Errorf("Expected 'fix-login-bug', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_Collision(t *testing.T) {
	mockStore := newMockCardStore()

	// Add existing card with alias
	mockStore.addCard("main", &model.Card{ID: "existing", Alias: "fix-bug"})

	service := NewAliasService(mockStore)

	alias, err := service.GenerateAlias("main", "Fix Bug")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	if alias != "fix-bug-2" {
		t.Errorf("Expected 'fix-bug-2', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_MultipleCollisions(t *testing.T) {
	mockStore := newMockCardStore()

	// Add existing cards with aliases
	mockStore.addCard("main", &model.Card{ID: "1", Alias: "fix-bug"})
	mockStore.addCard("main", &model.Card{ID: "2", Alias: "fix-bug-2"})
	mockStore.addCard("main", &model.Card{ID: "3", Alias: "fix-bug-3"})

	service := NewAliasService(mockStore)

	alias, err := service.GenerateAlias("main", "Fix Bug")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	if alias != "fix-bug-4" {
		t.Errorf("Expected 'fix-bug-4', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_EmptyTitle(t *testing.T) {
	mockStore := newMockCardStore()
	service := NewAliasService(mockStore)

	alias, err := service.GenerateAlias("main", "")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	// Should default to "card" for empty titles
	if alias != "card" {
		t.Errorf("Expected 'card', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_SpecialChars(t *testing.T) {
	mockStore := newMockCardStore()
	service := NewAliasService(mockStore)

	tests := []struct {
		title    string
		expected string
	}{
		{"Fix: login issue", "fix-login-issue"},
		{"Add feature (v2)", "add-feature-v2"},
		{"API v2.0 release", "api-v2-0-release"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
	}

	for _, tt := range tests {
		alias, err := service.GenerateAlias("main", tt.title)
		if err != nil {
			t.Fatalf("GenerateAlias(%q) failed: %v", tt.title, err)
		}
		if alias != tt.expected {
			t.Errorf("GenerateAlias(%q) = %q, want %q", tt.title, alias, tt.expected)
		}
	}
}

func TestAliasService_IsAliasAvailable(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "1", Alias: "taken-alias"})

	service := NewAliasService(mockStore)

	if service.IsAliasAvailable("main", "taken-alias") {
		t.Error("Alias should not be available")
	}

	if !service.IsAliasAvailable("main", "available-alias") {
		t.Error("Alias should be available")
	}
}

func TestAliasService_DifferentBoards(t *testing.T) {
	mockStore := newMockCardStore()

	// Same alias in different boards should be fine
	mockStore.addCard("board1", &model.Card{ID: "1", Alias: "fix-bug"})

	service := NewAliasService(mockStore)

	// Should be available in board2
	alias, err := service.GenerateAlias("board2", "Fix Bug")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	if alias != "fix-bug" {
		t.Errorf("Expected 'fix-bug' (available in different board), got %q", alias)
	}
}
