package cli

import (
	"os"

	"github.com/amterp/ra"
)

// CommandContext holds parsed values and used flags for all commands.
type CommandContext struct {
	// Global flags
	NonInteractive *bool

	// init command
	InitUsed     *bool
	InitLocation *string

	// board command
	BoardUsed       *bool
	BoardCreateUsed *bool
	BoardCreateName *string
	BoardListUsed   *bool

	// add command
	AddUsed        *bool
	AddTitle       *string
	AddDescription *string
	AddBoard       *string
	AddColumn      *string
	AddLabels      *[]string
	AddParent      *string

	// show command
	ShowUsed  *bool
	ShowCard  *string
	ShowBoard *string

	// list command
	ListUsed   *bool
	ListBoard  *string
	ListColumn *string

	// edit command
	EditUsed        *bool
	EditCard        *string
	EditBoard       *string
	EditTitle       *string
	EditDescription *string
	EditColumn      *string
	EditLabels      *[]string
	EditParent      *string
	EditAlias       *string
	EditFields      *[]string

	// serve command
	ServeUsed   *bool
	ServePort   *int
	ServeNoOpen *bool

	// migrate command
	MigrateUsed   *bool
	MigrateDryRun *bool
}

// Run is the main entry point for the CLI.
func Run() {
	ctx := &CommandContext{}

	cmd := ra.NewCmd("kan")
	cmd.SetDescription("File-based kanban boards")

	// Global flag for non-interactive mode
	ctx.NonInteractive, _ = ra.NewBool("non-interactive").
		SetShort("I").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Fail instead of prompting for missing input").
		Register(cmd, ra.WithGlobal(true))

	// Register all subcommands
	registerInit(cmd, ctx)
	registerBoard(cmd, ctx)
	registerAdd(cmd, ctx)
	registerShow(cmd, ctx)
	registerList(cmd, ctx)
	registerEdit(cmd, ctx)
	registerServe(cmd, ctx)
	registerMigrate(cmd, ctx)

	// Parse command line
	cmd.ParseOrExit(os.Args[1:])

	// Execute the appropriate command
	executeCommand(ctx)
}

func executeCommand(ctx *CommandContext) {
	switch {
	case *ctx.InitUsed:
		runInit(*ctx.InitLocation)

	case *ctx.BoardCreateUsed:
		runBoardCreate(*ctx.BoardCreateName)

	case *ctx.BoardListUsed:
		runBoardList()

	case *ctx.AddUsed:
		runAdd(*ctx.AddTitle, *ctx.AddDescription, *ctx.AddBoard, *ctx.AddColumn, *ctx.AddLabels, *ctx.AddParent, *ctx.NonInteractive)

	case *ctx.ShowUsed:
		runShow(*ctx.ShowCard, *ctx.ShowBoard)

	case *ctx.ListUsed:
		runList(*ctx.ListBoard, *ctx.ListColumn)

	case *ctx.EditUsed:
		runEdit(*ctx.EditCard, *ctx.EditBoard, *ctx.EditTitle, *ctx.EditDescription,
			*ctx.EditColumn, *ctx.EditLabels, *ctx.EditParent, *ctx.EditAlias,
			*ctx.EditFields, *ctx.NonInteractive)

	case *ctx.ServeUsed:
		runServe(*ctx.ServePort, *ctx.ServeNoOpen)

	case *ctx.MigrateUsed:
		runMigrate(*ctx.MigrateDryRun)
	}
}
