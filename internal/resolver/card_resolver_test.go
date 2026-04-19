package resolver

import (
	stderrors "errors"
	"strings"
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

// ============================================================================
// ResolveAcrossBoards Tests
// ============================================================================

func TestCardResolver_ResolveAcrossBoards_FoundInOneBoard(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "abc123", Title: "Fix Bug"})
	mockStore.addCard("feature", &model.Card{ID: "def456", Title: "New Feature"})

	resolver := NewCardResolver(mockStore)

	matches, err := resolver.ResolveAcrossBoards([]string{"main", "feature"}, "abc123")
	if err != nil {
		t.Fatalf("ResolveAcrossBoards failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}
	if matches[0].BoardName != "main" {
		t.Errorf("Expected board 'main', got %q", matches[0].BoardName)
	}
	if matches[0].Card.ID != "abc123" {
		t.Errorf("Expected card ID 'abc123', got %q", matches[0].Card.ID)
	}
}

func TestCardResolver_ResolveAcrossBoards_FoundInMultipleBoards(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "abc123", Title: "Main Bug"})
	mockStore.addCard("feature", &model.Card{ID: "abc123", Title: "Feature Bug"})

	resolver := NewCardResolver(mockStore)

	matches, err := resolver.ResolveAcrossBoards([]string{"main", "feature"}, "abc123")
	if err != nil {
		t.Fatalf("ResolveAcrossBoards failed: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("Expected 2 matches, got %d", len(matches))
	}
}

func TestCardResolver_ResolveAcrossBoards_NotFound(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "abc123", Title: "Fix Bug"})

	resolver := NewCardResolver(mockStore)

	matches, err := resolver.ResolveAcrossBoards([]string{"main", "feature"}, "nonexistent")
	if err != nil {
		t.Fatalf("ResolveAcrossBoards failed: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("Expected 0 matches, got %d", len(matches))
	}
}

func TestCardResolver_ResolveAcrossBoards_ByAlias(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "abc123", Alias: "fix-bug", Title: "Fix Bug"})
	mockStore.addCard("feature", &model.Card{ID: "def456", Alias: "new-feat", Title: "New Feature"})

	resolver := NewCardResolver(mockStore)

	matches, err := resolver.ResolveAcrossBoards([]string{"main", "feature"}, "fix-bug")
	if err != nil {
		t.Fatalf("ResolveAcrossBoards failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}
	if matches[0].BoardName != "main" {
		t.Errorf("Expected board 'main', got %q", matches[0].BoardName)
	}
}

// ============================================================================
// Fuzzy Resolve Tests
// ============================================================================

func TestCardResolver_Resolve_FuzzyUniqueMatch(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "abc123", Alias: "allow-fuzzy-matching"})
	mockStore.addCard("main", &model.Card{ID: "def456", Alias: "unrelated"})

	resolver := NewCardResolver(mockStore)
	card, err := resolver.Resolve("main", "allow-fuz")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if card.ID != "abc123" {
		t.Errorf("Expected card ID 'abc123', got %q", card.ID)
	}
}

func TestCardResolver_Resolve_FuzzyCaseInsensitive(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "abc123", Alias: "allow-fuzzy-matching"})

	resolver := NewCardResolver(mockStore)
	card, err := resolver.Resolve("main", "ALLOW-FUZ")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if card.ID != "abc123" {
		t.Errorf("Expected card ID 'abc123', got %q", card.ID)
	}
}

func TestCardResolver_Resolve_FuzzyMatchesID(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "deadbeef", Alias: "something-else"})

	resolver := NewCardResolver(mockStore)
	card, err := resolver.Resolve("main", "deadb")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if card.ID != "deadbeef" {
		t.Errorf("Expected card ID 'deadbeef', got %q", card.ID)
	}
}

func TestCardResolver_Resolve_FuzzyAmbiguous(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "id1", Alias: "allow-fuzzy-matching"})
	mockStore.addCard("main", &model.Card{ID: "id2", Alias: "allow-fuzzy-search"})

	resolver := NewCardResolver(mockStore)
	_, err := resolver.Resolve("main", "allow-fuz")
	if err == nil {
		t.Fatal("Expected ambiguous error, got nil")
	}
	if !kanerr.IsAmbiguous(err) {
		t.Fatalf("Expected ambiguous error, got %v", err)
	}
	var ambig *kanerr.AmbiguousCardError
	if !stderrors.As(err, &ambig) {
		t.Fatalf("Expected *AmbiguousCardError, got %T", err)
	}
	if len(ambig.Matches) != 2 {
		t.Errorf("Expected len(Matches)=2; got %d", len(ambig.Matches))
	}
	if ambig.Input != "allow-fuz" {
		t.Errorf("Expected Input=%q, got %q", "allow-fuz", ambig.Input)
	}
}

func TestCardResolver_Resolve_FuzzyAmbiguousCapped(t *testing.T) {
	mockStore := newMockCardStore()
	for i, alias := range []string{
		"fuzzy-a", "fuzzy-b", "fuzzy-c", "fuzzy-d",
		"fuzzy-e", "fuzzy-f", "fuzzy-g",
	} {
		mockStore.addCard("main", &model.Card{ID: string(rune('a' + i)), Alias: alias})
	}

	resolver := NewCardResolver(mockStore)
	_, err := resolver.Resolve("main", "fuzzy")
	if err == nil {
		t.Fatal("Expected ambiguous error, got nil")
	}
	var ambig *kanerr.AmbiguousCardError
	if !stderrors.As(err, &ambig) {
		t.Fatalf("Expected *AmbiguousCardError, got %T", err)
	}
	if len(ambig.Matches) != 7 {
		t.Errorf("Expected all 7 matches retained on error, got %d", len(ambig.Matches))
	}
	if ambig.DisplayLimit != 5 {
		t.Errorf("Expected DisplayLimit=5, got %d", ambig.DisplayLimit)
	}
	// Error() should render 5 rows + "(showing 5 of 7)" trailer.
	msg := ambig.Error()
	if !strings.Contains(msg, "(showing 5 of 7)") {
		t.Errorf("Expected trailer '(showing 5 of 7)' in error message, got:\n%s", msg)
	}
	// Should be sorted alphabetically (all are prefix matches).
	expected := []string{"fuzzy-a", "fuzzy-b", "fuzzy-c", "fuzzy-d", "fuzzy-e", "fuzzy-f", "fuzzy-g"}
	for i, want := range expected {
		if ambig.Matches[i].Alias != want {
			t.Errorf("Match %d: expected alias %q, got %q", i, want, ambig.Matches[i].Alias)
		}
	}
}

func TestCardResolver_Resolve_FuzzyRankingPrefixFirst(t *testing.T) {
	mockStore := newMockCardStore()
	// Midstring match comes alphabetically first, but prefix match must rank ahead.
	mockStore.addCard("main", &model.Card{ID: "id1", Alias: "alpha-fuz-thing"})
	mockStore.addCard("main", &model.Card{ID: "id2", Alias: "fuz-cleanup"})

	resolver := NewCardResolver(mockStore)
	_, err := resolver.Resolve("main", "fuz")
	if err == nil {
		t.Fatal("Expected ambiguous error")
	}
	var ambig *kanerr.AmbiguousCardError
	if !stderrors.As(err, &ambig) {
		t.Fatalf("Expected *AmbiguousCardError, got %T", err)
	}
	if ambig.Matches[0].Alias != "fuz-cleanup" {
		t.Errorf("Expected prefix match 'fuz-cleanup' first, got %q", ambig.Matches[0].Alias)
	}
}

func TestCardResolver_Resolve_ExactAliasBeatsFuzzy(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "id1", Alias: "foo"})
	mockStore.addCard("main", &model.Card{ID: "id2", Alias: "foobar"})

	resolver := NewCardResolver(mockStore)
	card, err := resolver.Resolve("main", "foo")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if card.ID != "id1" {
		t.Errorf("Expected exact-alias card 'id1', got %q", card.ID)
	}
}

func TestCardResolver_Resolve_FuzzyBelowMinQueryLen(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "id1", Alias: "abcd-something"})
	mockStore.addCard("main", &model.Card{ID: "id2", Alias: "abce-something"})

	resolver := NewCardResolver(mockStore)
	_, err := resolver.Resolve("main", "ab")
	if err == nil {
		t.Fatal("Expected not-found error for short query")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound for short query, got %v", err)
	}
}

func TestCardResolver_Resolve_FuzzyNoMatch(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "id1", Alias: "foo"})

	resolver := NewCardResolver(mockStore)
	_, err := resolver.Resolve("main", "nope")
	if err == nil {
		t.Fatal("Expected not-found error")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound, got %v", err)
	}
}

func TestCardResolver_ResolveAcrossBoards_FuzzyUniqueInOneBoard(t *testing.T) {
	mockStore := newMockCardStore()
	mockStore.addCard("main", &model.Card{ID: "abc123", Alias: "allow-fuzzy-matching"})
	mockStore.addCard("feature", &model.Card{ID: "def456", Alias: "unrelated"})

	resolver := NewCardResolver(mockStore)
	matches, err := resolver.ResolveAcrossBoards([]string{"main", "feature"}, "allow-fuz")
	if err != nil {
		t.Fatalf("ResolveAcrossBoards failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}
	if matches[0].Card.ID != "abc123" || matches[0].BoardName != "main" {
		t.Errorf("Unexpected match: %+v", matches[0])
	}
}

func TestCardResolver_ResolveAcrossBoards_FuzzyAmbiguousFlattens(t *testing.T) {
	mockStore := newMockCardStore()
	// Single board has 2 fuzzy matches - should flatten, not error.
	mockStore.addCard("main", &model.Card{ID: "id1", Alias: "allow-fuzzy-matching"})
	mockStore.addCard("main", &model.Card{ID: "id2", Alias: "allow-fuzzy-search"})

	resolver := NewCardResolver(mockStore)
	matches, err := resolver.ResolveAcrossBoards([]string{"main"}, "allow-fuz")
	if err != nil {
		t.Fatalf("ResolveAcrossBoards failed: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("Expected 2 flattened matches, got %d", len(matches))
	}
}

// ============================================================================
// ResolveAcrossBoards Tests (continued)
// ============================================================================

func TestCardResolver_ResolveAcrossBoards_EmptyBoardList(t *testing.T) {
	mockStore := newMockCardStore()
	resolver := NewCardResolver(mockStore)

	matches, err := resolver.ResolveAcrossBoards([]string{}, "abc123")
	if err != nil {
		t.Fatalf("ResolveAcrossBoards failed: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("Expected 0 matches, got %d", len(matches))
	}
}
