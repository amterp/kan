package cli

import (
	"testing"

	"github.com/amterp/kan/internal/model"
)

func TestColumnHistory_FiltersToColumnFieldInOrder(t *testing.T) {
	card := &model.Card{
		History: []model.HistoryEntry{
			{Field: "column", Value: "backlog", At: 100},
			{Field: "priority", Value: "high", At: 150}, // future field type - ignored
			{Field: "column", Value: "review", At: 200},
		},
	}

	got := columnHistory(card)
	if len(got) != 2 {
		t.Fatalf("expected 2 column entries, got %d", len(got))
	}
	if got[0].Value != "backlog" || got[1].Value != "review" {
		t.Errorf("unexpected entries (order/values): %+v", got)
	}
}

func TestColumnHistory_EmptyWhenNoHistory(t *testing.T) {
	if got := columnHistory(&model.Card{}); len(got) != 0 {
		t.Errorf("expected no entries, got %+v", got)
	}
}
