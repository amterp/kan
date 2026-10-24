package service

import (
	"fmt"

	"github.com/amterp/kan/internal/model"
)

// ReadyOptions are a ready query's command-line inputs.
type ReadyOptions struct {
	// Exclude holds the command's own exclusion filters, already validated.
	Exclude []model.Filter
	// NoBoardExclude skips the board's [ready] exclude list but keeps Exclude.
	NoBoardExclude bool
	// Limit caps the ready list; 0 means no cap.
	Limit int
}

// ReadyEntry is an open card with every one of its blockers looked up.
type ReadyEntry struct {
	Card     *model.Card
	Blockers []LinkLookup
}

// DroppedBlockers returns the blockers found in a dropped column.
func (e ReadyEntry) DroppedBlockers() []LinkLookup {
	var dropped []LinkLookup
	for _, b := range e.Blockers {
		if blockerCategory(b) == model.CategoryDropped {
			dropped = append(dropped, b)
		}
	}
	return dropped
}

// ReadyResult lists a board's ready cards and the open cards whose blocker
// was dropped, each in the order kan ready reads the board.
type ReadyResult struct {
	Board     *model.BoardConfig
	Ready     []ReadyEntry
	Attention []ReadyEntry
}

// FindReady answers a ready query for one board from an already loaded link
// graph, so the board, its cards and every blocker come from one read.
func FindReady(graph *LinkGraph, boardName string, opts ReadyOptions) (*ReadyResult, error) {
	board, err := graph.Board(boardName)
	if err != nil {
		return nil, err
	}
	cards, err := graph.Cards(boardName)
	if err != nil {
		return nil, err
	}
	return findReady(board, cards, func(card *model.Card) []LinkLookup {
		return graph.Blockers(boardName, card)
	}, opts)
}

func findReady(board *model.BoardConfig, cards []*model.Card, blockers func(*model.Card) []LinkLookup, opts ReadyOptions) (*ReadyResult, error) {
	if !board.HasCategories() {
		return nil, fmt.Errorf("board %q has no column categories, so kan can't tell which cards "+
			"are open work. Run \"kan column categorize\" to set them.", board.Name)
	}

	exclusions := model.FilterSet{Exclude: opts.Exclude}
	if !opts.NoBoardExclude {
		exclusions.Exclude = append(board.ReadyExclusions(), opts.Exclude...)
	}

	byColumn := make(map[string][]*model.Card)
	hasUnfinishedChild := make(map[string]bool)
	for _, card := range cards {
		byColumn[card.Column] = append(byColumn[card.Column], card)
		if card.Parent != "" && !isFinished(board.ColumnCategory(card.Column)) {
			hasUnfinishedChild[card.Parent] = true
		}
	}

	result := &ReadyResult{Board: board, Ready: []ReadyEntry{}, Attention: []ReadyEntry{}}
	for _, col := range board.ReadyColumns() {
		colCards := byColumn[col.Name]
		model.SortCardsByPosition(colCards)
		for _, card := range colCards {
			entry := ReadyEntry{Card: card, Blockers: blockers(card)}
			if len(entry.DroppedBlockers()) > 0 {
				result.Attention = append(result.Attention, entry)
			}
			if allBlockersDone(entry.Blockers) && !hasUnfinishedChild[card.ID] && exclusions.Matches(card, board) {
				result.Ready = append(result.Ready, entry)
			}
		}
	}

	if opts.Limit > 0 && len(result.Ready) > opts.Limit {
		result.Ready = result.Ready[:opts.Limit]
	}
	return result, nil
}

// isFinished reports whether a card in a column of this category no longer
// holds its parent back. An uncategorized column counts as unfinished.
func isFinished(category string) bool {
	return category == model.CategoryDone || category == model.CategoryDropped
}

// blockerCategory returns the category of the column a blocker was found in,
// or empty when it wasn't found or its column has no category.
func blockerCategory(b LinkLookup) string {
	if b.State != LinkFound || b.Column == nil {
		return ""
	}
	return b.Column.Category
}

func allBlockersDone(blockers []LinkLookup) bool {
	for _, b := range blockers {
		if blockerCategory(b) != model.CategoryDone {
			return false
		}
	}
	return true
}
