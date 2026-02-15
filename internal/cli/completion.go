package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/discovery"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/resolver"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/ra"
)

// completionCtx provides lightweight store access for shell completion.
// Completion functions run during ParseOrExit, before NewApp() is called,
// so we can't use the full App. This initializes just enough to list boards,
// cards, and columns.
type completionCtx struct {
	once        sync.Once
	paths       *config.Paths
	boardStore  *store.FileBoardStore
	cardStore   *store.FileCardStore
	globalCfg   *model.GlobalConfig
	projectRoot string
	err         error
}

var compCtx completionCtx

func initCompletionCtx() {
	compCtx.once.Do(func() {
		globalStore := store.NewGlobalStore()
		globalCfg, err := globalStore.Load()
		if err != nil {
			// Graceful degradation: no completions if global config is broken
			globalCfg = nil
		}
		compCtx.globalCfg = globalCfg

		result, err := discovery.DiscoverProject(globalCfg)
		if err != nil || result == nil {
			compCtx.err = fmt.Errorf("no project found")
			return
		}

		compCtx.projectRoot = result.ProjectRoot
		compCtx.paths = config.NewPaths(result.ProjectRoot, result.DataLocation)
		compCtx.boardStore = store.NewBoardStore(compCtx.paths)
		compCtx.cardStore = store.NewCardStore(compCtx.paths)
	})
}

// completeBoards returns board names matching the given prefix.
func completeBoards(toComplete string) ([]string, ra.CompletionDirective) {
	initCompletionCtx()
	if compCtx.err != nil {
		return nil, ra.CompletionDirectiveNoFileComp
	}

	boards, err := compCtx.boardStore.List()
	if err != nil {
		return nil, ra.CompletionDirectiveNoFileComp
	}

	var result []string
	for _, b := range boards {
		if strings.HasPrefix(b, toComplete) {
			result = append(result, b)
		}
	}
	return result, ra.CompletionDirectiveNoFileComp
}

// completeCards returns card IDs and aliases matching the given prefix.
func completeCards(toComplete string) ([]string, ra.CompletionDirective) {
	initCompletionCtx()
	if compCtx.err != nil {
		return nil, ra.CompletionDirectiveNoFileComp
	}

	board := hintBoard()
	if board == "" {
		return nil, ra.CompletionDirectiveNoFileComp
	}

	cards, err := compCtx.cardStore.List(board)
	if err != nil {
		return nil, ra.CompletionDirectiveNoFileComp
	}

	var result []string
	for _, card := range cards {
		if strings.HasPrefix(card.ID, toComplete) {
			result = append(result, card.ID)
		}
		if card.Alias != "" && strings.HasPrefix(card.Alias, toComplete) {
			result = append(result, card.Alias)
		}
	}
	return result, ra.CompletionDirectiveNoFileComp
}

// completeColumns returns column names matching the given prefix.
func completeColumns(toComplete string) ([]string, ra.CompletionDirective) {
	initCompletionCtx()
	if compCtx.err != nil {
		return nil, ra.CompletionDirectiveNoFileComp
	}

	board := hintBoard()
	if board == "" {
		return nil, ra.CompletionDirectiveNoFileComp
	}

	boardCfg, err := compCtx.boardStore.Get(board)
	if err != nil {
		return nil, ra.CompletionDirectiveNoFileComp
	}

	var result []string
	for _, col := range boardCfg.Columns {
		if strings.HasPrefix(col.Name, toComplete) {
			result = append(result, col.Name)
		}
	}
	return result, ra.CompletionDirectiveNoFileComp
}

// hintBoard resolves which board to use for completion context.
// Scans os.Args for -b/--board, falls back to resolver.InferBoard
// for single-board auto-detect and default board from global config.
func hintBoard() string {
	if board := boardFromArgs(os.Args); board != "" {
		return board
	}
	return resolver.InferBoard(compCtx.boardStore, compCtx.globalCfg, compCtx.projectRoot)
}

// boardFromArgs scans the argument list for an explicit -b/--board flag value.
func boardFromArgs(args []string) string {
	for i, arg := range args {
		// --board=value or -b=value (skip empty values so fallback logic runs)
		if strings.HasPrefix(arg, "--board=") {
			if v := strings.TrimPrefix(arg, "--board="); v != "" {
				return v
			}
		}
		if strings.HasPrefix(arg, "-b=") {
			if v := strings.TrimPrefix(arg, "-b="); v != "" {
				return v
			}
		}
		// --board value or -b value
		if (arg == "--board" || arg == "-b") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// registerCompletion adds the "kan completion <shell>" command.
func registerCompletion(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("completion")
	cmd.SetDescription("Output shell completion script")

	ctx.CompletionShell, _ = ra.NewString("shell").
		SetUsage("Shell type").
		SetEnumConstraint([]string{"bash", "zsh"}).
		Register(cmd)

	ctx.CompletionUsed, _ = parent.RegisterCmd(cmd)
}

// runCompletion outputs the shell completion script to stdout.
func runCompletion(shell string, rootCmd *ra.Cmd) {
	var err error
	switch shell {
	case "bash":
		err = rootCmd.GenBashCompletion(os.Stdout)
	case "zsh":
		err = rootCmd.GenZshCompletion(os.Stdout)
	default:
		Fatal(fmt.Errorf("unsupported shell: %s (supported: bash, zsh)", shell))
	}
	if err != nil {
		Fatal(fmt.Errorf("failed to generate completion script: %w", err))
	}
}
