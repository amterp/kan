package cli

import (
	"fmt"

	"github.com/amterp/ra"
)

func registerLink(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("link")
	cmd.SetDescription("Add blockers or related cards")
	ctx.LinkCard, _ = ra.NewString("card").SetUsage("Card ID or alias").SetCompletionFunc(completeCards).Register(cmd)
	ctx.LinkBlockedBy, _ = ra.NewStringSlice("blocked-by").SetOptional(true).SetFlagOnly(true).SetVariadic(true).
		SetUsage("Cards that must finish first (repeatable)").SetCompletionFunc(completeCards).Register(cmd)
	ctx.LinkRelated, _ = ra.NewStringSlice("related").SetOptional(true).SetFlagOnly(true).SetVariadic(true).
		SetUsage("Related cards (repeatable)").SetCompletionFunc(completeCards).Register(cmd)
	ctx.LinkBoard, _ = ra.NewString("board").SetShort("b").SetOptional(true).SetFlagOnly(true).
		SetUsage("Board name or ID").SetCompletionFunc(completeBoards).Register(cmd)
	ctx.LinkGlobal = registerGlobalFlag(cmd)
	ctx.LinkUsed, _ = parent.RegisterCmd(cmd)
}

func registerUnlink(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("unlink")
	cmd.SetDescription("Remove card links, including dangling references")
	ctx.UnlinkCard, _ = ra.NewString("card").SetUsage("Card ID or alias").SetCompletionFunc(completeCards).Register(cmd)
	ctx.UnlinkRefs, _ = ra.NewStringSlice("refs").SetVariadic(true).SetUsage("Stored refs, card IDs, or aliases").
		SetCompletionFunc(completeCards).Register(cmd)
	ctx.UnlinkBoard, _ = ra.NewString("board").SetShort("b").SetOptional(true).SetFlagOnly(true).
		SetUsage("Board name or ID").SetCompletionFunc(completeBoards).Register(cmd)
	ctx.UnlinkGlobal = registerGlobalFlag(cmd)
	ctx.UnlinkUsed, _ = parent.RegisterCmd(cmd)
}

func runLink(cardRef, board string, blockedBy, related []string, global, nonInteractive, jsonOutput bool) {
	if len(blockedBy) == 0 && len(related) == 0 {
		Fatal(fmt.Errorf("provide --blocked-by or --related"))
	}
	app, err := NewAppWithOptions(AppOptions{Interactive: !nonInteractive, UseGlobalBoard: global})
	if err != nil {
		Fatal(err)
	}
	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}
	target, err := app.ResolveCardWithBoard(board, cardRef, !nonInteractive, forWrite)
	if err != nil {
		Fatal(err)
	}
	card, err := app.LinkService.Link(target.BoardName, target.Card.ID, blockedBy, related)
	if err != nil {
		Fatal(err)
	}
	if jsonOutput {
		if err := printJson(NewCardOutput(card)); err != nil {
			Fatal(err)
		}
		return
	}
	PrintSuccess("Updated links for %s", card.ID)
}

func runUnlink(cardRef, board string, refs []string, global, nonInteractive, jsonOutput bool) {
	if len(refs) == 0 {
		Fatal(fmt.Errorf("provide at least one reference to remove"))
	}
	app, err := NewAppWithOptions(AppOptions{Interactive: !nonInteractive, UseGlobalBoard: global})
	if err != nil {
		Fatal(err)
	}
	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}
	target, err := app.ResolveCardWithBoard(board, cardRef, !nonInteractive, forWrite)
	if err != nil {
		Fatal(err)
	}
	result, err := app.LinkService.UnlinkWithResult(target.BoardName, target.Card.ID, refs)
	if err != nil {
		Fatal(err)
	}
	for _, ref := range result.Unmatched {
		PrintWarning("No link matched %q", ref)
	}
	if result.Removed == 0 {
		Fatal(fmt.Errorf("no matching links to remove"))
	}
	if jsonOutput {
		if err := printJson(NewCardOutput(result.Card)); err != nil {
			Fatal(err)
		}
		return
	}
	PrintSuccess("Removed %d link(s) from %s", result.Removed, result.Card.ID)
}
