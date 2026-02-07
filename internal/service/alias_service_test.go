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

	// "Fix Bug" has only 2 words, no more to expand, falls back to -2
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

func TestAliasService_GenerateAlias_ProgressiveExpansion(t *testing.T) {
	mockStore := newMockCardStore()

	// "update-authentication" is 21 chars (exceeds threshold) but gets used
	// because minSlugWords=2 is always enforced. Collide with it to test expansion.
	mockStore.addCard("main", &model.Card{ID: "1", Alias: "update-authentication"})

	service := NewAliasService(mockStore)

	// Should expand to include the next word rather than going to -2
	alias, err := service.GenerateAlias("main", "Update authentication middleware")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	if alias != "update-authentication-middleware" {
		t.Errorf("Expected 'update-authentication-middleware', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_ProgressiveExpansionMultipleSteps(t *testing.T) {
	mockStore := newMockCardStore()

	// Collide both the base and the first expansion
	mockStore.addCard("main", &model.Card{ID: "1", Alias: "update-authentication"})
	mockStore.addCard("main", &model.Card{ID: "2", Alias: "update-authentication-middleware"})

	service := NewAliasService(mockStore)

	alias, err := service.GenerateAlias("main", "Update authentication middleware layer")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	if alias != "update-authentication-middleware-layer" {
		t.Errorf("Expected 'update-authentication-middleware-layer', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_WordExhaustion(t *testing.T) {
	mockStore := newMockCardStore()

	// Collide the base and the only expansion available
	mockStore.addCard("main", &model.Card{ID: "1", Alias: "update-authentication"})
	mockStore.addCard("main", &model.Card{ID: "2", Alias: "update-authentication-middleware"})

	service := NewAliasService(mockStore)

	// Title only has 3 words, all expansions collide, should fall back to -2
	alias, err := service.GenerateAlias("main", "Update authentication middleware")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	if alias != "update-authentication-2" {
		t.Errorf("Expected 'update-authentication-2', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_LongTitleTruncated(t *testing.T) {
	mockStore := newMockCardStore()
	service := NewAliasService(mockStore)

	// Long title should be truncated to threshold
	alias, err := service.GenerateAlias("main", "This is a very long title that exceeds the limit")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	// "this-is-a-very-long" = 19 chars, adding "title" would be 25 > 20
	if alias != "this-is-a-very-long" {
		t.Errorf("Expected 'this-is-a-very-long', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_LongTitleCollisionExpands(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "1", Alias: "this-is-a-very-long"})

	service := NewAliasService(mockStore)

	alias, err := service.GenerateAlias("main", "This is a very long title that exceeds the limit")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	// On collision, next word "title" is added
	if alias != "this-is-a-very-long-title" {
		t.Errorf("Expected 'this-is-a-very-long-title', got %q", alias)
	}
}

func TestAliasService_GenerateAlias_SingleWord(t *testing.T) {
	mockStore := newMockCardStore()
	service := NewAliasService(mockStore)

	alias, err := service.GenerateAlias("main", "Bug")
	if err != nil {
		t.Fatalf("GenerateAlias failed: %v", err)
	}

	if alias != "bug" {
		t.Errorf("Expected 'bug', got %q", alias)
	}
}

func TestWordsForThreshold(t *testing.T) {
	tests := []struct {
		name     string
		words    []string
		expected int
	}{
		{"short two words", []string{"fix", "bug"}, 2},
		{"three short words", []string{"fix", "login", "bug"}, 3},
		{"min words enforced", []string{"update", "authentication", "middleware"}, 2},
		{"single word", []string{"bug"}, 1},
		{"many short words", []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}, 10},
		{"threshold boundary", []string{"this", "is", "a", "very", "long"}, 5}, // "this-is-a-very-long" = 19
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wordsForThreshold(tt.words)
			if result != tt.expected {
				t.Errorf("wordsForThreshold(%v) = %d, want %d", tt.words, result, tt.expected)
			}
		})
	}
}
