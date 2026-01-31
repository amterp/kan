package cli

import (
	"fmt"
	"sort"
	"strings"

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
		printCardsList(cards, boardCfg)
	}
}

// cardColumnWidths holds the calculated widths for aligning card output.
type cardColumnWidths struct {
	idWidth   int
	typeWidth int
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

		// Calculate column widths for alignment within this column
		widths := calculateColumnWidths(colCards, boardCfg)
		for _, card := range colCards {
			printCardLine(card, boardCfg, widths)
		}
	}
}

func printCardsList(cards []*model.Card, boardCfg *model.BoardConfig) {
	widths := calculateColumnWidths(cards, boardCfg)
	for _, card := range cards {
		printCardLine(card, boardCfg, widths)
	}
}

// getTypeIndicatorValue returns the type indicator field value for a card, or empty string.
func getTypeIndicatorValue(card *model.Card, boardCfg *model.BoardConfig) string {
	typeField := boardCfg.CardDisplay.TypeIndicator
	if typeField == "" {
		return ""
	}
	if val, ok := card.CustomFields[typeField].(string); ok {
		return val
	}
	return ""
}

// calculateColumnWidths calculates the max widths for ID and type indicator columns.
func calculateColumnWidths(cards []*model.Card, boardCfg *model.BoardConfig) cardColumnWidths {
	widths := cardColumnWidths{}
	for _, card := range cards {
		if len(card.ID) > widths.idWidth {
			widths.idWidth = len(card.ID)
		}
		val := getTypeIndicatorValue(card, boardCfg)
		if val != "" {
			// Visual width is len("[" + val + "]")
			width := len(val) + 2
			if width > widths.typeWidth {
				widths.typeWidth = width
			}
		}
	}
	return widths
}

func printCardLine(card *model.Card, boardCfg *model.BoardConfig, widths cardColumnWidths) {
	// Render ID with padding
	idPadding := widths.idWidth - len(card.ID)
	renderedID := RenderID(card.ID) + strings.Repeat(" ", idPadding)

	// Render type indicator with padding
	typeIndicator := ""
	if widths.typeWidth > 0 {
		val := getTypeIndicatorValue(card, boardCfg)
		if val != "" {
			color := boardCfg.GetOptionColor(boardCfg.CardDisplay.TypeIndicator, val)
			rendered := RenderTypeIndicator(val, color)
			// Pad to align: visual width is len(val) + 2 for brackets
			padding := widths.typeWidth - (len(val) + 2)
			typeIndicator = rendered + strings.Repeat(" ", padding) + "  "
		} else {
			// No value but others have type indicators, maintain alignment
			typeIndicator = strings.Repeat(" ", widths.typeWidth) + "  "
		}
	}
	fmt.Printf("  %s  %s%s\n", renderedID, typeIndicator, card.Title)
}
