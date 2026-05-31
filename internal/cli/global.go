package cli

import (
	"errors"
	"fmt"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/ra"
)

// registerGlobalFlag adds the shared -g/--global flag to a command. When set,
// the command targets the designated global board (see `kan global set`)
// instead of the project discovered from the working directory.
func registerGlobalFlag(cmd *ra.Cmd) *bool {
	flag, _ := ra.NewBool("global").
		SetShort("g").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target the designated global board, -b still overrides (see 'kan global set')").
		Register(cmd)
	return flag
}

func registerGlobal(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("global")
	cmd.SetDescription("Manage the global board reachable from anywhere via -g")

	// global set
	setCmd := ra.NewCmd("set")
	setCmd.SetDescription("Designate the current project's board as the global board (run from inside a project)")

	ctx.GlobalSetBoard, _ = ra.NewString("board").
		SetOptional(true).
		SetUsage("Board to designate (defaults to the resolved board)").
		SetCompletionFunc(completeBoards).
		Register(setCmd)

	ctx.GlobalSetUsed, _ = cmd.RegisterCmd(setCmd)

	// global show
	showCmd := ra.NewCmd("show")
	showCmd.SetDescription("Show the current global board designation")
	ctx.GlobalShowUsed, _ = cmd.RegisterCmd(showCmd)

	// global unset
	unsetCmd := ra.NewCmd("unset")
	unsetCmd.SetDescription("Clear the global board designation")
	ctx.GlobalUnsetUsed, _ = cmd.RegisterCmd(unsetCmd)

	ctx.GlobalUsed, _ = parent.RegisterCmd(cmd)
}

func runGlobalSet(boardArg string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}
	if err := app.RequireKan(); err != nil {
		// 'global set' records the *current* project's board, so it must run from
		// inside one - give a pointed message rather than the generic init error.
		var notInit *kanerr.NotInitializedError
		if errors.As(err, &notInit) {
			Fatal(fmt.Errorf("'kan global set' must be run from inside a kan project; cd to the project whose board you want to make global"))
		}
		Fatal(err)
	}

	// Reuse normal board resolution: explicit arg wins, else single-board
	// auto-detect / default / interactive picker.
	boardName, err := app.BoardResolver.Resolve(boardArg, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	globalCfg, err := app.GlobalStore.Load()
	if err != nil {
		Fatal(err)
	}
	globalCfg.SetGlobalBoard(app.ProjectRoot, boardName)
	if err := app.GlobalStore.Save(globalCfg); err != nil {
		Fatal(err)
	}

	PrintSuccess("Global board set to %q %s", boardName, RenderMuted("("+prettyPath(app.ProjectRoot)+")"))
	PrintInfo("Use it from anywhere with -g, e.g. 'kan add -g \"...\"'")
}

func runGlobalShow() {
	cfg, err := loadGlobalConfig()
	if err != nil {
		Fatal(err)
	}

	if cfg.GlobalBoard == nil {
		PrintInfo("No global board set. Run 'kan global set' from a project to designate one.")
		return
	}

	ref := cfg.GlobalBoard
	PrintInfo("Global board: %q %s", ref.Board, RenderMuted("("+prettyPath(ref.Path)+")"))

	// Surface staleness proactively so 'show' doubles as a health check.
	var dataLocation string
	if repoCfg := cfg.GetRepoConfig(ref.Path); repoCfg != nil {
		dataLocation = repoCfg.DataLocation
	}
	boardStore := store.NewBoardStore(config.NewPaths(ref.Path, dataLocation))
	if !boardStore.Exists(ref.Board) {
		PrintWarning("board %q no longer exists at that location; run 'kan global set' to re-designate or 'kan global unset' to clear it", ref.Board)
	}
}

func runGlobalUnset() {
	cfg, err := loadGlobalConfig()
	if err != nil {
		Fatal(err)
	}

	if cfg.GlobalBoard == nil {
		PrintInfo("No global board set.")
		return
	}

	cfg.ClearGlobalBoard()
	if err := store.NewGlobalStore().Save(cfg); err != nil {
		Fatal(err)
	}

	PrintSuccess("Cleared the global board designation")
}

// loadGlobalConfig loads the global config (auto-migrating it first) without
// requiring a project in the working directory - global show/unset work from
// anywhere, including outside any kan project.
func loadGlobalConfig() (*model.GlobalConfig, error) {
	if err := autoMigrateGlobal(); err != nil {
		// Best effort: a future-version error still surfaces below on Load.
		PrintWarning("failed to auto-migrate global config: %v", err)
	}
	return store.NewGlobalStore().Load()
}
