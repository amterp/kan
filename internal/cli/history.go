package cli

import (
	"fmt"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/util"
	"github.com/amterp/ra"
)

func registerHistory(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("history")
	cmd.SetDescription("Show a card's column transition history")

	ctx.HistoryCard, _ = ra.NewString("card").
		SetUsage("Card ID or alias").
		SetCompletionFunc(completeCards).
		Register(cmd)

	ctx.HistoryBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		SetCompletionFunc(completeBoards).
		Register(cmd)

	ctx.HistoryUsed, _ = parent.RegisterCmd(cmd)
}

// historyOutput is the --json shape for `kan history`.
type historyOutput struct {
	Card    string               `json:"card"`
	Board   string               `json:"board,omitempty"`
	History []model.HistoryEntry `json:"history"`
}

func runHistory(idOrAlias, board string, jsonOutput bool) {
	app, err := NewApp(true)
	if err != nil {
		Fatal(err)
	}
	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	result, err := app.ResolveCardWithBoard(board, idOrAlias, true)
	if err != nil {
		Fatal(err)
	}
	boardName := result.BoardName
	card := result.Card

	columnEntries := columnHistory(card)

	if jsonOutput {
		out := historyOutput{Card: card.ID, History: columnEntries}
		if result.MultipleBoards {
			out.Board = boardName
		}
		if err := printJson(out); err != nil {
			Fatal(err)
		}
		return
	}

	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	fmt.Println(TitleBox(card.Title))
	fmt.Println()

	if len(columnEntries) == 0 {
		fmt.Println(RenderMuted("No column history recorded."))
		return
	}

	now := util.NowMillis()
	for i, entry := range columnEntries {
		column := fmt.Sprintf("%v", entry.Value)

		// Each value is held until the next column entry, or until now for the
		// latest one (which is the card's current column).
		end := now
		isCurrent := true
		if i+1 < len(columnEntries) {
			end = columnEntries[i+1].At
			isCurrent = false
		}
		duration := util.FormatDuration(end - entry.At)

		line := fmt.Sprintf("%s  %s  %s",
			RenderColumnColor(column, columnColor(boardCfg, column)),
			RenderMuted(util.FormatMillis(entry.At)),
			duration,
		)
		if isCurrent {
			line += " " + RenderMuted("(current)")
		}
		fmt.Println(line)
	}
}

// columnHistory returns the card's history entries that track column changes,
// in chronological order. Other tracked fields (future) are filtered out.
// Always returns a non-nil slice so --json emits `[]` rather than `null`.
func columnHistory(card *model.Card) []model.HistoryEntry {
	entries := make([]model.HistoryEntry, 0, len(card.History))
	for _, e := range card.History {
		if e.Field == "column" {
			entries = append(entries, e)
		}
	}
	return entries
}

// columnColor looks up a column's configured color, returning "" if not found
// (e.g. the card sits in a column that was since renamed or removed).
func columnColor(boardCfg *model.BoardConfig, column string) string {
	for _, col := range boardCfg.Columns {
		if col.Name == column {
			return col.Color
		}
	}
	return ""
}
