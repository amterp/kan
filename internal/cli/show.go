package cli

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/util"
	"github.com/amterp/ra"
)

func registerShow(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("show")
	cmd.SetDescription("Display card details")

	ctx.ShowCard, _ = ra.NewString("card").
		SetUsage("Card ID or alias").
		Register(cmd)

	ctx.ShowBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		Register(cmd)

	ctx.ShowUsed, _ = parent.RegisterCmd(cmd)
}

func runShow(idOrAlias, board string, jsonOutput bool) {
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

	// Resolve card
	card, err := app.CardResolver.Resolve(boardName, idOrAlias)
	if err != nil {
		Fatal(err)
	}

	// Get column from board config (not stored in card file)
	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}
	card.Column = boardCfg.GetCardColumn(card.ID)

	if jsonOutput {
		if err := printJson(NewCardOutput(card)); err != nil {
			Fatal(err)
		}
		return
	}

	printCard(card)
}

func printCard(card *model.Card) {
	fmt.Printf("ID:      %s\n", card.ID)
	fmt.Printf("Alias:   %s\n", card.Alias)
	fmt.Printf("Title:   %s\n", card.Title)
	fmt.Printf("Column:  %s\n", card.Column)

	if card.Description != "" {
		fmt.Printf("Description:\n  %s\n", strings.ReplaceAll(card.Description, "\n", "\n  "))
	}

	if card.Parent != "" {
		fmt.Printf("Parent:  %s\n", card.Parent)
	}

	fmt.Printf("Creator: %s\n", card.Creator)
	fmt.Printf("Created: %s\n", util.FormatMillis(card.CreatedAtMillis))
	fmt.Printf("Updated: %s\n", util.FormatMillis(card.UpdatedAtMillis))

	if len(card.Comments) > 0 {
		fmt.Printf("\nComments (%d):\n", len(card.Comments))
		for _, comment := range card.Comments {
			fmt.Printf("  [%s] %s:\n", util.FormatMillis(comment.CreatedAtMillis), comment.Author)
			fmt.Printf("    %s\n", strings.ReplaceAll(comment.Body, "\n", "\n    "))
		}
	}

	if len(card.CustomFields) > 0 {
		fmt.Println("\nCustom Fields:")
		for k, v := range card.CustomFields {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}
}
