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

	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	card, err := app.CardResolver.Resolve(boardName, cardArg)
	if err != nil {
		Fatal(err)
	}

	if err := app.CardService.Delete(boardName, card.ID); err != nil {
		Fatal(err)
	}

	PrintSuccess("Deleted card %q (%s) from board %q", card.Title, card.ID, boardName)
}
