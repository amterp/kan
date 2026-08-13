package cli

import (
	"fmt"

	"github.com/amterp/ra"
)

func registerMove(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("move")
	cmd.SetDescription("Move a card to a different column")

	ctx.MoveCard, _ = ra.NewString("card").
		SetUsage("Card ID or alias").
		SetCompletionFunc(completeCards).
		Register(cmd)

	// Optional so `kan move <card> --after <other>` can infer the column from
	// the anchor, matching how --before/--after behave on add and edit.
	ctx.MoveColumn, _ = ra.NewString("column").
		SetOptional(true).
		SetUsage("Destination column (inferred from --before/--after if omitted)").
		SetCompletionFunc(completeColumns).
		Register(cmd)

	ctx.MovePosition, ctx.MoveTop, ctx.MoveBottom, ctx.MoveBefore, ctx.MoveAfter =
		registerPlacementFlags(cmd, "Move", "the column argument")

	ctx.MoveBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		SetCompletionFunc(completeBoards).
		Register(cmd)

	ctx.MoveGlobal = registerGlobalFlag(cmd)

	ctx.MoveUsed, _ = parent.RegisterCmd(cmd)
}

func runMove(cardArg, column, board string, placement cardPlacement,
	global, nonInteractive, jsonOutput bool) {

	if column == "" && placement.before == "" && placement.after == "" {
		Fatal(fmt.Errorf("nothing to move to: give a column, or use --before/--after to " +
			"place the card relative to another one"))
	}

	app, err := NewAppWithOptions(AppOptions{Interactive: !nonInteractive, UseGlobalBoard: global})
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	result, err := app.ResolveCardWithBoard(board, cardArg, !nonInteractive)
	if err != nil {
		Fatal(err)
	}
	boardName := result.BoardName
	card := result.Card
	app.PrintGlobalTarget(boardName)

	if result.CrossBoard {
		PrintInfo("Found card in board %q", boardName)
	}

	// The card cannot anchor to itself.
	position, beforeID, afterID, err := resolvePlacement(app, boardName, card.ID, placement)
	if err != nil {
		Fatal(err)
	}

	if err := app.CardService.MoveCardWithPlacement(
		boardName, card.ID, column, position, beforeID, afterID); err != nil {
		Fatal(err)
	}

	// Re-fetch so the output reflects the move rather than the stale pre-move card.
	movedCard, err := app.CardService.Get(boardName, card.ID)
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		output := NewCardOutput(movedCard)
		output.Card.Board = boardName
		if err := printJson(output); err != nil {
			Fatal(err)
		}
		return
	}

	PrintSuccess("Moved card %s to %q", RenderID(movedCard.ID), movedCard.Column)
}
