package cli

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/util"
	"github.com/amterp/ra"
)

func registerShow(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("show")
	cmd.SetDescription("Display card details")

	ctx.ShowCard, _ = ra.NewString("card").
		SetUsage("Card ID, alias, or board:card").
		SetCompletionFunc(completeCards).
		Register(cmd)

	ctx.ShowBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name or ID").
		SetCompletionFunc(completeBoards).
		Register(cmd)

	ctx.ShowGlobal = registerGlobalFlag(cmd)

	ctx.ShowUsed, _ = parent.RegisterCmd(cmd)
}

func runShow(idOrAlias, board string, global, jsonOutput bool) {
	app, err := NewAppWithOptions(AppOptions{Interactive: true, UseGlobalBoard: global, ReadOnly: true})
	if err != nil {
		Fatal(err)
	}

	// Resolve board and card together (with cross-board search)
	result, err := app.ResolveCardWithBoard(board, idOrAlias, true, forRead)
	if err != nil {
		Fatal(err)
	}
	boardName := result.BoardName
	card := result.Card
	project := result.Project
	project.PrintTarget(boardName)

	// Don't print CrossBoard info for show - the Board field in card output covers it

	boardCfg, err := project.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		output := NewCardOutput(card)
		output.Card.Board = boardName
		output.Project = otherProjectPath(project)
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
	printCard(card, colColor, boardName, result.MultipleBoards || project.OtherProject)
	links, err := service.LoadLinkGraph(project.BoardStore, project.CardStore)
	if err != nil {
		Fatal(err)
	}
	links.SetElsewhere(app.elsewhere(project))
	for _, skipped := range links.LoadOtherProjects() {
		PrintWarning("skipped %s while looking for cards that link here: %s", util.PrettyPath(skipped.Path), skipped.Reason)
	}
	printCardLinks(links, boardName, card)
}

func printCardLinks(links *service.LinkGraph, boardName string, card *model.Card) {
	printLinks := func(label string, results []service.LinkLookup) {
		if len(results) == 0 {
			return
		}
		fmt.Printf("\n%s:\n", label)
		for _, result := range results {
			if result.State == service.LinkBoardNotHere && label == "Blocked by" {
				fmt.Printf("  %s\n", strings.ReplaceAll(service.NotOnThisMachine(card.ID, result), "\n", "\n  "))
				continue
			}
			if result.State != service.LinkFound {
				fmt.Printf("  %s: %s (%s)\n", result.Ref, result.State, result.Reason)
				continue
			}
			state := "uncategorized"
			if result.Column != nil && result.Column.Category != "" {
				state = result.Column.Category
			}
			// A card on another board is named by its stored form, and one in
			// another project also by where that project is.
			ref, where := result.Card.ID, ""
			if result.BoardName != boardName || result.Project != "" {
				ref = result.BoardID + ":" + result.Card.ID
			}
			if result.Project != "" {
				where = "  " + RenderMuted(util.PrettyPath(result.Project))
			}
			fmt.Printf("  %s  %s (%s, %s)%s\n", ref, result.Card.Title, result.Card.Column, state, where)
		}
	}
	printLinks("Blocked by", links.Blockers(boardName, card))
	printLinks("Blocks", links.Blocks(boardName, card.ID))
	var related []service.LinkLookup
	for _, ref := range card.Related {
		related = append(related, links.Resolve(boardName, ref))
	}
	printLinks("Related", related)
	printLinks("Related to this card", links.RelatedTo(boardName, card.ID))
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

	// Show how long the card has been in its current column - the time git
	// can't tell you accurately because it only knows your commit cadence.
	colDuration := util.FormatDuration(util.NowMillis() - card.CurrentColumnSinceMillis())
	columnValue := RenderColumnColor(card.Column, colColor) + " " + RenderMuted("("+colDuration+")")
	fmt.Println(LabelValue("Column", columnValue, labelWidth))

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
