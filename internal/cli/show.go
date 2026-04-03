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
		SetCompletionFunc(completeCards).
		Register(cmd)

	ctx.ShowBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		SetCompletionFunc(completeBoards).
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

	// Resolve board and card together (with cross-board search)
	result, err := app.ResolveCardWithBoard(board, idOrAlias, true)
	if err != nil {
		Fatal(err)
	}
	boardName := result.BoardName
	card := result.Card

	// Don't print CrossBoard info for show - the Board field in card output covers it

	// Get column from board config (not stored in card file)
	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}
	card.Column = boardCfg.GetCardColumn(card.ID)

	if jsonOutput {
		output := NewCardOutput(card)
		output.Card.Board = boardName
		if err := printJson(output); err != nil {
			Fatal(err)
		}
		return
	}

	// Get column color for display
	var colColor string
	for _, col := range boardCfg.Columns {
		if col.Name == card.Column {
			colColor = col.Color
			break
		}
	}
	printCard(card, colColor, boardName, result.MultipleBoards)
}

func printCard(card *model.Card, colColor, boardName string, multipleBoards bool) {
	const labelWidth = 10

	// Title box
	fmt.Println(TitleBox(card.Title))
	fmt.Println()

	// Card details with aligned labels
	fmt.Println(LabelValue("ID", RenderID(card.ID), labelWidth))
	if multipleBoards {
		fmt.Println(LabelValue("Board", boardName, labelWidth))
	}
	fmt.Println(LabelValue("Alias", card.Alias, labelWidth))
	fmt.Println(LabelValue("Column", RenderColumnColor(card.Column, colColor), labelWidth))

	if card.Description != "" {
		fmt.Println()
		fmt.Println(RenderMuted("Description:"))
		fmt.Printf("  %s\n", strings.ReplaceAll(card.Description, "\n", "\n  "))
	}

	if card.Parent != "" {
		fmt.Println(LabelValue("Parent", RenderID(card.Parent), labelWidth))
	}

	fmt.Println()
	fmt.Println(LabelValue("Creator", card.Creator, labelWidth))
	fmt.Println(LabelValue("Created", RenderMuted(util.FormatMillis(card.CreatedAtMillis)), labelWidth))
	fmt.Println(LabelValue("Updated", RenderMuted(util.FormatMillis(card.UpdatedAtMillis)), labelWidth))

	if len(card.Comments) > 0 {
		fmt.Printf("\n%s\n", RenderMuted(fmt.Sprintf("Comments (%d):", len(card.Comments))))
		for _, comment := range card.Comments {
			timestamp := RenderMuted(fmt.Sprintf("[%s]", util.FormatMillis(comment.CreatedAtMillis)))
			fmt.Printf("  %s %s:\n", timestamp, RenderBold(comment.Author))
			fmt.Printf("    %s\n", strings.ReplaceAll(comment.Body, "\n", "\n    "))
		}
	}

	if len(card.CustomFields) > 0 {
		fmt.Printf("\n%s\n", RenderMuted("Custom Fields:"))
		for k, v := range card.CustomFields {
			fmt.Printf("  %s: %v\n", RenderMuted(k), v)
		}
	}
}
