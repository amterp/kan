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

	ctx.ReadyAll, _ = ra.NewBool("all").
		SetOptional(true).
		SetFlagOnly(true).
		SetExcludes([]string{"board", "global"}).
		SetUsage("List ready cards on every board of every visible registered project (--limit applies per board)").
		Register(cmd)

	ctx.ReadyUsed, _ = parent.RegisterCmd(cmd)
}

func runReady(board string, excludes []string, noExclude bool, limit int, global, all, jsonOutput bool) {
	app, err := NewAppWithOptions(AppOptions{Interactive: true, UseGlobalBoard: global, ReadOnly: true})
	if err != nil {
		Fatal(err)
	}

	exclude, err := model.ParseFilters(excludes)
	if err != nil {
		Fatal(err)
	}
	opts := service.ReadyOptions{Exclude: exclude, NoBoardExclude: noExclude, Limit: limit}

	if all {
		runReadyAll(app, opts, jsonOutput)
		return
	}

	result, err := readyOne(app, board, opts)
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		if err := printJson(NewReadyOutput([]readyBoard{result}, nil)); err != nil {
			Fatal(err)
		}
		return
	}
	if len(result.Result.Ready) == 0 {
		PrintInfo("No cards are ready")
	}
	printReady(result.Result)
}

// readyOne runs a ready query on the board -b names, which may be another
// registered project's board, resolved the way kan list resolves it.
func readyOne(app *App, board string, opts service.ReadyOptions) (readyBoard, error) {
	project, boardName, err := app.ResolveBoard(board, true, forRead)
	if err != nil {
		return readyBoard{}, err
	}
	project.PrintTarget(boardName)

	graph, err := service.LoadLinkGraph(project.BoardStore, project.CardStore)
	if err != nil {
		return readyBoard{}, err
	}
	graph.SetElsewhere(app.elsewhere(project))
	boardCfg, err := graph.Board(boardName)
	if err != nil {
		return readyBoard{}, err
	}
	if err := model.ValidateFilters(opts.Exclude, boardCfg); err != nil {
		return readyBoard{}, err
	}
	warnUnreadCards(graph, boardName)

	result, err := service.FindReady(graph, boardName, opts)
	if err != nil {
		return readyBoard{}, err
	}
	return readyBoard{ProjectPath: project.ProjectRoot, Result: result}, nil
}

func runReadyAll(app *App, opts service.ReadyOptions, jsonOutput bool) {
	boards, skipped, err := readyAll(app, opts)
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		if err := printJson(NewReadyOutput(boards, skipped)); err != nil {
			Fatal(err)
		}
		return
	}

	printed, anyReady := 0, false
	for _, b := range boards {
		if len(b.Result.Ready) == 0 && len(b.Result.Attention) == 0 {
			continue
		}
		printBoardHeader(b.Result.Board.Name, b.ProjectPath, printed == 0)
		printed++
		anyReady = anyReady || len(b.Result.Ready) > 0
		printReady(b.Result)
	}
	if !anyReady {
		PrintInfo("No cards are ready")
	}
	printSkipped(skipped)
}

// readyAll runs a ready query on every board --all reads, loading each
// project's link graph once. A board without column categories is skipped,
// not an error, since kan can't tell which of its cards are open work.
func readyAll(app *App, opts service.ReadyOptions) ([]readyBoard, []skippedBoard, error) {
	targets, skipped := app.boardsForAll()

	type loaded struct {
		allBoard
		graph *service.LinkGraph
		cfg   *model.BoardConfig
	}
	graphs := map[*App]*service.LinkGraph{}
	var boards []loaded
	var configs []*model.BoardConfig
	for _, t := range targets {
		graph := graphs[t.Project]
		if graph == nil {
			var err error
			if graph, err = service.LoadLinkGraph(t.Project.BoardStore, t.Project.CardStore); err != nil {
				skipped = skipBoard(skipped, t, "", err)
				continue
			}
			graph.SetElsewhere(app.elsewhere(t.Project))
			graphs[t.Project] = graph
		}
		cfg, err := graph.Board(t.Name)
		if err != nil {
			skipped = skipBoard(skipped, t, "", err)
			continue
		}
		boards = append(boards, loaded{allBoard: t, graph: graph, cfg: cfg})
		configs = append(configs, cfg)
	}
	if len(configs) > 0 {
		if err := model.ValidateFilters(opts.Exclude, configs...); err != nil {
			return nil, nil, err
		}
	}

	var results []readyBoard
	for _, b := range boards {
		if !b.cfg.HasCategories() {
			skipped = append(skipped, skippedBoard{
				Path: b.Project.ProjectRoot, BoardID: b.cfg.ID,
				Reason: fmt.Sprintf("board %q has no column categories; run \"kan column categorize\" in that project",
					b.Name),
			})
			continue
		}
		warnUnreadCards(b.graph, b.Name)
		result, err := service.FindReady(b.graph, b.Name, opts)
		if err != nil {
			skipped = skipBoard(skipped, b.allBoard, b.cfg.ID, err)
			continue
		}
		results = append(results, readyBoard{ProjectPath: b.Project.ProjectRoot, Result: result})
	}
	sortSkipped(skipped)
	return results, skipped, nil
}

func warnUnreadCards(graph *service.LinkGraph, boardName string) {
	for _, failure := range graph.Failures(boardName) {
		PrintWarning("skipping unreadable card file %s.json: %v. kan ready can't consider that card, "+
			"or tell whether it holds back a parent. Run \"kan doctor\" to check the board", failure.CardID, failure.Err)
	}
}

func printReady(result *service.ReadyResult) {
	board := result.Board

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
