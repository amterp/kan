package service

import (
	"reflect"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

func readyTestBoard() *model.BoardConfig {
	return &model.BoardConfig{
		Name: "main",
		Columns: []model.Column{
			{Name: "later", Category: model.CategoryOpen},
			{Name: "next", Category: model.CategoryOpen},
			{Name: "doing", Category: model.CategoryActive},
			{Name: "done", Category: model.CategoryDone},
			{Name: "dropped", Category: model.CategoryDropped},
			{Name: "ideas"},
		},
		CustomFields: map[string]model.CustomFieldSchema{
			"labels": {Type: model.FieldTypeEnumSet},
		},
	}
}

// blockerIn returns a found blocker in the named column, which may not exist.
func blockerIn(board *model.BoardConfig, column string) LinkLookup {
	return LinkLookup{State: LinkFound, Ref: "blocker-in-" + column, Column: board.GetColumn(column)}
}

func readyIDs(entries []ReadyEntry) []string {
	ids := []string{}
	for _, e := range entries {
		ids = append(ids, e.Card.ID)
	}
	return ids
}

func runFindReady(t *testing.T, board *model.BoardConfig, cards []*model.Card, blockers map[string][]LinkLookup, opts ReadyOptions) *ReadyResult {
	t.Helper()
	result, err := findReady(board, cards, func(c *model.Card) []LinkLookup { return blockers[c.ID] }, opts)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFindReady_Blockers(t *testing.T) {
	board := readyTestBoard()
	cases := []struct {
		name      string
		blocker   LinkLookup
		ready     bool
		attention bool
	}{
		{"found in done", blockerIn(board, "done"), true, false},
		{"found in an open column", blockerIn(board, "next"), false, false},
		{"found in an active column", blockerIn(board, "doing"), false, false},
		{"found in dropped", blockerIn(board, "dropped"), false, true},
		{"found in an uncategorized column", blockerIn(board, "ideas"), false, false},
		{"found in a column the board no longer has", blockerIn(board, "gone"), false, false},
		{"absent", LinkLookup{State: LinkAbsent, Ref: "x"}, false, false},
		{"unreadable", LinkLookup{State: LinkUnreadable, Ref: "x"}, false, false},
		{"board not here", LinkLookup{State: LinkBoardNotHere, Ref: "b:x"}, false, false},
		{"unsupported schema", LinkLookup{State: LinkUnsupportedSchema, Ref: "x"}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cards := []*model.Card{{ID: "a", Column: "next"}}
			result := runFindReady(t, board, cards, map[string][]LinkLookup{
				"a": {blockerIn(board, "done"), tc.blocker},
			}, ReadyOptions{})
			if got := len(result.Ready) == 1; got != tc.ready {
				t.Errorf("ready = %v, want %v", got, tc.ready)
			}
			if got := len(result.Attention) == 1; got != tc.attention {
				t.Errorf("attention = %v, want %v", got, tc.attention)
			}
		})
	}
}

func TestFindReady_OnlyOpenCardsAndUnfinishedChildren(t *testing.T) {
	board := readyTestBoard()
	cards := []*model.Card{
		{ID: "doing-card", Column: "doing"},
		{ID: "idea-card", Column: "ideas"},
		{ID: "lone", Column: "next", Position: "a"},
		{ID: "parent-done-kids", Column: "next", Position: "b"},
		{ID: "kid-done", Column: "done", Parent: "parent-done-kids"},
		{ID: "kid-dropped", Column: "dropped", Parent: "parent-done-kids"},
	}
	for _, unfinished := range []string{"later", "doing", "ideas", "gone"} {
		parent := "parent-kid-in-" + unfinished
		cards = append(cards,
			&model.Card{ID: parent, Column: "next", Position: "c"},
			&model.Card{ID: "kid-in-" + unfinished, Column: unfinished, Parent: parent},
		)
	}
	result := runFindReady(t, board, cards, nil, ReadyOptions{})
	// kid-in-later is itself ready: it is an open card with no blockers.
	want := []string{"lone", "parent-done-kids", "kid-in-later"}
	if got := readyIDs(result.Ready); !reflect.DeepEqual(got, want) {
		t.Errorf("ready = %v, want %v", got, want)
	}
}

func TestFindReady_Order(t *testing.T) {
	board := readyTestBoard()
	cards := []*model.Card{
		{ID: "later-1", Column: "later", Position: "a"},
		{ID: "next-2", Column: "next", Position: "b"},
		{ID: "next-1", Column: "next", Position: "a"},
	}
	result := runFindReady(t, board, cards, nil, ReadyOptions{})
	if got, want := readyIDs(result.Ready), []string{"next-1", "next-2", "later-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("right to left: %v, want %v", got, want)
	}

	board.Ready.Order = []string{"later"}
	result = runFindReady(t, board, cards, nil, ReadyOptions{})
	if got, want := readyIDs(result.Ready), []string{"later-1", "next-1", "next-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("with [ready] order: %v, want %v", got, want)
	}
}

func TestFindReady_ExclusionsAndLimit(t *testing.T) {
	board := readyTestBoard()
	board.Ready.Exclude = []string{"labels=needs-decision", "nofield=x"}
	cards := []*model.Card{
		{ID: "plain", Column: "next", Position: "a"},
		{ID: "undecided", Column: "next", Position: "b", CustomFields: map[string]any{"labels": []any{"needs-decision"}}},
		{ID: "urgent", Column: "next", Position: "c", CustomFields: map[string]any{"labels": []any{"urgent"}}},
		{ID: "undecided-dropped", Column: "next", Position: "d", CustomFields: map[string]any{"labels": []any{"needs-decision"}}},
		{ID: "dropped-2", Column: "next", Position: "e"},
	}
	blockers := map[string][]LinkLookup{
		"undecided-dropped": {blockerIn(board, "dropped")},
		"dropped-2":         {blockerIn(board, "dropped")},
	}
	urgent := []model.Filter{{Key: "labels", Value: "urgent"}}

	cases := []struct {
		name string
		opts ReadyOptions
		want []string
	}{
		{"board exclusions, with the undefined field skipped", ReadyOptions{}, []string{"plain", "urgent"}},
		{"plus the command's", ReadyOptions{Exclude: urgent}, []string{"plain"}},
		{"--no-exclude keeps the command's", ReadyOptions{Exclude: urgent, NoBoardExclude: true}, []string{"plain", "undecided"}},
		{"limit caps ready", ReadyOptions{Limit: 1}, []string{"plain"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runFindReady(t, board, cards, blockers, tc.opts)
			if got := readyIDs(result.Ready); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ready = %v, want %v", got, tc.want)
			}
			// Attention ignores exclusions and the limit.
			if got, want := readyIDs(result.Attention), []string{"undecided-dropped", "dropped-2"}; !reflect.DeepEqual(got, want) {
				t.Errorf("attention = %v, want %v", got, want)
			}
		})
	}
}

func TestFindReady_Categories(t *testing.T) {
	_, err := findReady(&model.BoardConfig{Name: "main", Columns: []model.Column{{Name: "todo"}}}, nil, nil, ReadyOptions{})
	want := `board "main" has no column categories, so kan can't tell which cards are open work. Run "kan column categorize" to set them.`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}

	noOpen := &model.BoardConfig{Name: "main", Columns: []model.Column{{Name: "done", Category: model.CategoryDone}}}
	result, err := findReady(noOpen, []*model.Card{{ID: "a", Column: "done"}}, nil, ReadyOptions{})
	if err != nil || len(result.Ready) != 0 || result.Attention == nil {
		t.Errorf("board without open columns: %+v, %v", result, err)
	}
}

func TestFindReady_FromStores(t *testing.T) {
	paths := config.NewPaths(t.TempDir(), "")
	boards := store.NewBoardStore(paths)
	cards := store.NewCardStore(paths)
	board := readyTestBoard()
	board.ID = "board-1"
	if err := boards.Create(board); err != nil {
		t.Fatal(err)
	}
	for _, card := range []*model.Card{
		{ID: "shipped", Column: "done"},
		{ID: "waiting", Column: "next", BlockedBy: []string{"shipped"}},
		{ID: "stuck", Column: "next", BlockedBy: []string{"missing"}},
	} {
		if err := cards.Create("main", card); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := LoadLinkGraph(boards, cards)
	if err != nil {
		t.Fatal(err)
	}
	result, err := FindReady(graph, "main", ReadyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := readyIDs(result.Ready); !reflect.DeepEqual(got, []string{"waiting"}) {
		t.Errorf("ready = %v, want [waiting]", got)
	}
	if b := result.Ready[0].Blockers; len(b) != 1 || b[0].State != LinkFound || b[0].Card.ID != "shipped" {
		t.Errorf("blockers = %+v", b)
	}

	if _, err := FindReady(graph, "other", ReadyOptions{}); err == nil || !strings.Contains(err.Error(), "other") {
		t.Errorf("unknown board error = %v", err)
	}
}
