package cli

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/ra"
)

func registerReady(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("ready")
	cmd.SetDescription("List open cards that can be started now")

	ctx.ReadyBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		SetCompletionFunc(completeBoards).
		Register(cmd)

	ctx.ReadyExclude, _ = ra.NewStringSlice("exclude").
		SetShort("x").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Also keep cards whose custom field matches key=value out of ready (repeatable)").
		SetCompletionFunc(completeFilters).
		Register(cmd)

	ctx.ReadyNoExclude, _ = ra.NewBool("no-exclude").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Ignore the board's [ready] exclude list (-x still applies)").
		Register(cmd)

	ctx.ReadyLimit, _ = ra.NewInt("limit").
		SetOptional(true).
		SetFlagOnly(true).
		SetMin(1, true).
		SetUsage("Print only the first N ready cards").
		Register(cmd)

	ctx.ReadyGlobal = registerGlobalFlag(cmd)

	ctx.ReadyUsed, _ = parent.RegisterCmd(cmd)
}

func runReady(board string, excludes []string, noExclude bool, limit int, global, jsonOutput bool) {
	app, err := NewAppWithOptions(AppOptions{Interactive: true, UseGlobalBoard: global, ReadOnly: true})
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	boardName, err := app.BoardResolver.Resolve(board, true)
	if err != nil {
		Fatal(err)
	}
	app.PrintGlobalTarget(boardName)

	graph, err := service.LoadLinkGraph(app.BoardStore, app.CardStore)
	if err != nil {
		Fatal(err)
	}
	boardCfg, err := graph.Board(boardName)
	if err != nil {
		Fatal(err)
	}

	exclude, err := model.ParseFilters(excludes)
	if err != nil {
		Fatal(err)
	}
	if err := model.ValidateFilters(exclude, boardCfg); err != nil {
		Fatal(err)
	}

	for _, failure := range graph.Failures(boardName) {
		PrintWarning("skipping unreadable card file %s.json: %v. kan ready can't consider that card, "+
			"or tell whether it holds back a parent. Run \"kan doctor\" to check the board", failure.CardID, failure.Err)
	}

	result, err := service.FindReady(graph, boardName, service.ReadyOptions{
		Exclude:        exclude,
		NoBoardExclude: noExclude,
		Limit:          limit,
	})
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		if err := printJson(NewReadyOutput(result, app.Paths.ProjectRoot())); err != nil {
			Fatal(err)
		}
		return
	}
	printReady(result)
}

func printReady(result *service.ReadyResult) {
	board := result.Board
	if len(result.Ready) == 0 {
		PrintInfo("No cards are ready")
	}

	var cards []*model.Card
	for _, e := range result.Ready {
		cards = append(cards, e.Card)
	}
	widths := calculateColumnWidths(cards, board)
	column := ""
	for _, e := range result.Ready {
		if e.Card.Column != column {
			column = e.Card.Column
			fmt.Println(RenderColumnColor(column, board.GetColumn(column).Color))
		}
		printCardLine(e.Card, board, widths)
	}

	if len(result.Attention) == 0 {
		return
	}
	fmt.Println()
	fmt.Println("Needs attention")
	for _, e := range result.Attention {
		var reasons []string
		for _, b := range e.DroppedBlockers() {
			reasons = append(reasons, fmt.Sprintf("%s (%s)", b.Ref, b.Card.Column))
		}
		fmt.Printf("  %s  %s  %s\n", RenderID(e.Card.ID), e.Card.Title,
			RenderMuted("blocker dropped: "+strings.Join(reasons, ", ")))
	}
}
