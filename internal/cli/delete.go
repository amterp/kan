package cli

import (
	"fmt"

	"github.com/amterp/ra"
)

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

	ctx.DeleteForce, _ = ra.NewBool("force").
		SetShort("f").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Skip confirmation (required in non-interactive mode)").
		Register(cmd)

	ctx.DeleteUsed, _ = parent.RegisterCmd(cmd)
}

func runDelete(cardArg, board string, force, nonInteractive bool) {
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

	if !force {
		if nonInteractive {
			Fatal(fmt.Errorf("deleting card %q (%s) requires --force in non-interactive mode", card.Title, card.ID))
		}

		confirmed, err := app.Prompter.Confirm(
			fmt.Sprintf("Delete card %q (%s)?", card.Title, card.ID),
			false,
		)
		if err != nil {
			Fatal(err)
		}
		if !confirmed {
			PrintInfo("Cancelled")
			return
		}
	}

	if err := app.CardService.Delete(boardName, card.ID); err != nil {
		Fatal(err)
	}

	PrintSuccess("Deleted card %q (%s) from board %q", card.Title, card.ID, boardName)
}
