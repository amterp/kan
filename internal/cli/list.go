package cli

import (
	"fmt"
	"sort"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/ra"
)

func registerList(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("list")
	cmd.SetDescription("List cards")

	ctx.ListBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Filter by board").
		Register(cmd)

	ctx.ListColumn, _ = ra.NewString("column").
		SetShort("c").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Filter by column").
		Register(cmd)

	ctx.ListUsed, _ = parent.RegisterCmd(cmd)
}

func runList(board, column string, jsonOutput bool) {
	app, err := NewApp(true)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	// Resolve board
	boardName, err := app.BoardResolver.Resolve(board, true)
	if err != nil {
		Fatal(err)
	}

	// Get board config for column ordering
	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	// Get cards
	cards, err := app.CardService.List(boardName, column)
	if err != nil {
		Fatal(err)
	}

	// Sort cards by column order, then by created_at
	columnOrder := make(map[string]int)
	for i, col := range boardCfg.Columns {
		columnOrder[col.Name] = i
	}

	sort.Slice(cards, func(i, j int) bool {
		ci, cj := cards[i], cards[j]
		if ci.Column != cj.Column {
			return columnOrder[ci.Column] < columnOrder[cj.Column]
		}
		return ci.CreatedAtMillis < cj.CreatedAtMillis
	})

	if jsonOutput {
		if err := printJson(NewListOutput(cards)); err != nil {
			Fatal(err)
		}
		return
	}

	if len(cards) == 0 {
		PrintInfo("No cards found")
		return
	}

	// Group by column if not filtering by column
	if column == "" {
		printCardsByColumn(cards, boardCfg)
	} else {
		printCardsList(cards)
	}
}

func printCardsByColumn(cards []*model.Card, boardCfg *model.BoardConfig) {
	cardsByColumn := make(map[string][]*model.Card)
	for _, card := range cards {
		cardsByColumn[card.Column] = append(cardsByColumn[card.Column], card)
	}

	for _, col := range boardCfg.Columns {
		colCards := cardsByColumn[col.Name]
		if len(colCards) == 0 {
			continue
		}

		// Column header with color from board config
		colHeader := RenderColumnColor(col.Name, col.Color)
		countStr := RenderMuted(fmt.Sprintf("(%d)", len(colCards)))
		fmt.Printf("\n%s %s\n", colHeader, countStr)
		for _, card := range colCards {
			printCardLine(card)
		}
	}
}

func printCardsList(cards []*model.Card) {
	for _, card := range cards {
		printCardLine(card)
	}
}

func printCardLine(card *model.Card) {
	fmt.Printf("  %s  %s\n", RenderID(card.ID), card.Title)
}
