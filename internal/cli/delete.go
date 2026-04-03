package cli

import "github.com/amterp/ra"

func registerDelete(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("delete")
	cmd.SetDescription("Delete a card")

	ctx.DeleteCard, _ = ra.NewString("card").
		SetUsage("Card ID or alias").
		SetCompletionFunc(completeCards).
		Register(cmd)

	ctx.DeleteBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		SetCompletionFunc(completeBoards).
		Register(cmd)

	ctx.DeleteUsed, _ = parent.RegisterCmd(cmd)
}

func runDelete(cardArg, board string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	// Resolve board and card together (with cross-board search)
	result, err := app.ResolveCardWithBoard(board, cardArg, !nonInteractive)
	if err != nil {
		Fatal(err)
	}
	boardName := result.BoardName
	card := result.Card

	if result.CrossBoard {
		PrintInfo("Found card in board %q", boardName)
	}

	if err := app.CardService.Delete(boardName, card.ID); err != nil {
		Fatal(err)
	}

	PrintSuccess("Deleted card %q (%s) from board %q", card.Title, card.ID, boardName)
}
