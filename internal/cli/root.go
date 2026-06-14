package cli

import (
	"os"

	"github.com/amterp/ra"
)

// CommandContext holds parsed values and used flags for all commands.
type CommandContext struct {
	// Global flags
	NonInteractive *bool
	Json           *bool

	// completion command
	CompletionUsed  *bool
	CompletionShell *string
	RootCmd         *ra.Cmd

	// init command
	InitUsed        *bool
	InitLocation    *string
	InitColumns     *string
	InitName        *string
	InitProjectName *string

	// board command
	BoardUsed       *bool
	BoardCreateUsed *bool
	BoardCreateName *string
	BoardListUsed   *bool

	// board describe
	BoardDescribeUsed  *bool
	BoardDescribeName  *string
	BoardDescribeBoard *string

	// delete command
	DeleteUsed   *bool
	DeleteCard   *string
	DeleteBoard  *string
	DeleteGlobal *bool

	// board delete
	BoardDeleteUsed *bool
	BoardDeleteName *string

	// add command
	AddUsed        *bool
	AddTitle       *string
	AddDescription *string
	AddBoard       *string
	AddColumn      *string
	AddParent      *string
	AddPosition    *int
	AddBefore      *string
	AddAfter       *string
	AddFields      *[]string
	AddStrict      *bool
	AddGlobal      *bool

	// show command
	ShowUsed   *bool
	ShowCard   *string
	ShowBoard  *string
	ShowGlobal *bool

	// history command
	HistoryUsed   *bool
	HistoryCard   *string
	HistoryBoard  *string
	HistoryGlobal *bool

	// list command
	ListUsed   *bool
	ListBoard  *string
	ListColumn *string
	ListGlobal *bool

	// edit command
	EditUsed        *bool
	EditCard        *string
	EditBoard       *string
	EditTitle       *string
	EditDescription *string
	EditColumn      *string
	EditParent      *string
	EditPosition    *int
	EditBefore      *string
	EditAfter       *string
	EditAlias       *string
	EditFields      *[]string
	EditStrict      *bool
	EditGlobal      *bool

	// serve command
	ServeUsed   *bool
	ServePort   *int
	ServeHost   *string
	ServeNoOpen *bool

	// migrate command
	MigrateUsed   *bool
	MigrateDryRun *bool
	MigrateAll    *bool

	// column command
	ColumnUsed *bool

	// column add
	ColumnAddUsed        *bool
	ColumnAddName        *string
	ColumnAddColor       *string
	ColumnAddDescription *string
	ColumnAddPosition    *int
	ColumnAddLimit       *int
	ColumnAddBoard       *string

	// column delete
	ColumnDeleteUsed  *bool
	ColumnDeleteName  *string
	ColumnDeleteBoard *string

	// column rename
	ColumnRenameUsed  *bool
	ColumnRenameOld   *string
	ColumnRenameNew   *string
	ColumnRenameBoard *string

	// column edit
	ColumnEditUsed        *bool
	ColumnEditName        *string
	ColumnEditColor       *string
	ColumnEditDescription *string
	ColumnEditLimit       *int
	ColumnEditBoard       *string

	// column list
	ColumnListUsed  *bool
	ColumnListBoard *string

	// column move
	ColumnMoveUsed     *bool
	ColumnMoveName     *string
	ColumnMovePosition *int
	ColumnMoveAfter    *string
	ColumnMoveBoard    *string

	// comment command
	CommentUsed *bool

	// comment add
	CommentAddUsed   *bool
	CommentAddCard   *string
	CommentAddBody   *string
	CommentAddBoard  *string
	CommentAddGlobal *bool

	// comment edit
	CommentEditUsed   *bool
	CommentEditID     *string
	CommentEditBody   *string
	CommentEditBoard  *string
	CommentEditGlobal *bool

	// comment delete
	CommentDeleteUsed   *bool
	CommentDeleteID     *string
	CommentDeleteBoard  *string
	CommentDeleteGlobal *bool

	// doctor command
	DoctorUsed   *bool
	DoctorFix    *bool
	DoctorDryRun *bool
	DoctorBoard  *string

	// commit command
	CommitUsed    *bool
	CommitMessage *string

	// global command
	GlobalUsed *bool

	// global set
	GlobalSetUsed  *bool
	GlobalSetBoard *string

	// global show
	GlobalShowUsed *bool

	// global unset
	GlobalUnsetUsed *bool
}

// Run is the main entry point for the CLI.
func Run() {
	ctx := buildRootCmd()

	// Parse command line
	ctx.RootCmd.ParseOrExit(os.Args[1:])

	// Execute the appropriate command
	executeCommand(ctx)
}

// buildRootCmd constructs the root command with all subcommands and flags
// registered. It is split out from Run so tests can parse argument slices and
// inspect parsed state (e.g. Configured) without triggering command execution.
func buildRootCmd() *CommandContext {
	ctx := &CommandContext{}

	cmd := ra.NewCmd("kan")
	cmd.SetDescription("File-based kanban boards")
	cmd.EnableCompletion()
	ctx.RootCmd = cmd

	// Global flag for non-interactive mode
	ctx.NonInteractive, _ = ra.NewBool("non-interactive").
		SetShort("I").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Fail instead of prompting for missing input").
		Register(cmd, ra.WithGlobal(true))

	// Global flag for JSON output
	ctx.Json, _ = ra.NewBool("json").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Output results as JSON").
		Register(cmd, ra.WithGlobal(true))

	// Register all subcommands
	registerInit(cmd, ctx)
	registerBoard(cmd, ctx)
	registerColumn(cmd, ctx)
	registerComment(cmd, ctx)
	registerAdd(cmd, ctx)
	registerDelete(cmd, ctx)
	registerShow(cmd, ctx)
	registerHistory(cmd, ctx)
	registerList(cmd, ctx)
	registerEdit(cmd, ctx)
	registerServe(cmd, ctx)
	registerMigrate(cmd, ctx)
	registerDoctor(cmd, ctx)
	registerCommit(cmd, ctx)
	registerGlobal(cmd, ctx)
	registerCompletion(cmd, ctx)

	return ctx
}

func executeCommand(ctx *CommandContext) {
	// Warn about --json on unsupported commands
	if *ctx.Json {
		unsupportedCommand := ""
		switch {
		case *ctx.InitUsed:
			unsupportedCommand = "init"
		case *ctx.BoardCreateUsed:
			unsupportedCommand = "board create"
		case *ctx.ServeUsed:
			unsupportedCommand = "serve"
		case *ctx.MigrateUsed:
			unsupportedCommand = "migrate"
		case *ctx.ColumnAddUsed:
			unsupportedCommand = "column add"
		case *ctx.ColumnDeleteUsed:
			unsupportedCommand = "column delete"
		case *ctx.ColumnRenameUsed:
			unsupportedCommand = "column rename"
		case *ctx.ColumnEditUsed:
			unsupportedCommand = "column edit"
		case *ctx.ColumnMoveUsed:
			unsupportedCommand = "column move"
		case *ctx.CommentEditUsed:
			unsupportedCommand = "comment edit"
		case *ctx.CommentDeleteUsed:
			unsupportedCommand = "comment delete"
		case *ctx.DeleteUsed:
			unsupportedCommand = "delete"
		case *ctx.BoardDeleteUsed:
			unsupportedCommand = "board delete"
		case *ctx.CommitUsed:
			unsupportedCommand = "commit"
		}
		if unsupportedCommand != "" {
			warnJsonNotSupported(unsupportedCommand)
		}
	}

	switch {
	case *ctx.InitUsed:
		runInit(*ctx.InitLocation, *ctx.InitName, *ctx.InitColumns, *ctx.InitProjectName, *ctx.NonInteractive)

	case *ctx.BoardCreateUsed:
		runBoardCreate(*ctx.BoardCreateName)

	case *ctx.BoardDeleteUsed:
		runBoardDelete(*ctx.BoardDeleteName, *ctx.NonInteractive)

	case *ctx.BoardDescribeUsed:
		runBoardDescribe(*ctx.BoardDescribeName, *ctx.BoardDescribeBoard, *ctx.NonInteractive, *ctx.Json)

	case *ctx.BoardListUsed:
		runBoardList(*ctx.Json)

	case *ctx.AddUsed:
		runAdd(*ctx.AddTitle, *ctx.AddDescription, *ctx.AddBoard, *ctx.AddColumn, *ctx.AddParent,
			cardPlacement{*ctx.AddPosition, ctx.RootCmd.Configured("position"), *ctx.AddBefore, *ctx.AddAfter},
			*ctx.AddFields, *ctx.AddStrict, *ctx.AddGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.DeleteUsed:
		runDelete(*ctx.DeleteCard, *ctx.DeleteBoard, *ctx.DeleteGlobal, *ctx.NonInteractive)

	case *ctx.ShowUsed:
		runShow(*ctx.ShowCard, *ctx.ShowBoard, *ctx.ShowGlobal, *ctx.Json)

	case *ctx.HistoryUsed:
		runHistory(*ctx.HistoryCard, *ctx.HistoryBoard, *ctx.HistoryGlobal, *ctx.Json)

	case *ctx.ListUsed:
		runList(*ctx.ListBoard, *ctx.ListColumn, *ctx.ListGlobal, *ctx.Json)

	case *ctx.EditUsed:
		runEdit(*ctx.EditCard, *ctx.EditBoard, *ctx.EditTitle, *ctx.EditDescription,
			*ctx.EditColumn, *ctx.EditParent, *ctx.EditAlias,
			cardPlacement{*ctx.EditPosition, ctx.RootCmd.Configured("position"), *ctx.EditBefore, *ctx.EditAfter},
			*ctx.EditFields, *ctx.EditStrict, *ctx.EditGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ServeUsed:
		runServe(*ctx.ServeHost, *ctx.ServePort, ctx.RootCmd.Configured("port"), *ctx.ServeNoOpen)

	case *ctx.MigrateUsed:
		if *ctx.MigrateAll {
			runMigrateAll(*ctx.MigrateDryRun, *ctx.NonInteractive)
		} else {
			runMigrate(*ctx.MigrateDryRun)
		}

	case *ctx.ColumnAddUsed:
		runColumnAdd(*ctx.ColumnAddName, *ctx.ColumnAddColor, *ctx.ColumnAddDescription, *ctx.ColumnAddPosition, *ctx.ColumnAddLimit, *ctx.ColumnAddBoard, *ctx.NonInteractive)

	case *ctx.ColumnDeleteUsed:
		runColumnDelete(*ctx.ColumnDeleteName, *ctx.ColumnDeleteBoard, *ctx.NonInteractive)

	case *ctx.ColumnRenameUsed:
		runColumnRename(*ctx.ColumnRenameOld, *ctx.ColumnRenameNew, *ctx.ColumnRenameBoard, *ctx.NonInteractive)

	case *ctx.ColumnEditUsed:
		runColumnEdit(*ctx.ColumnEditName, *ctx.ColumnEditColor, *ctx.ColumnEditDescription, *ctx.ColumnEditLimit, *ctx.ColumnEditBoard, *ctx.NonInteractive)

	case *ctx.ColumnListUsed:
		runColumnList(*ctx.ColumnListBoard, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ColumnMoveUsed:
		runColumnMove(*ctx.ColumnMoveName, *ctx.ColumnMoveBoard, *ctx.ColumnMovePosition, *ctx.ColumnMoveAfter, *ctx.NonInteractive)

	case *ctx.CommentAddUsed:
		runCommentAdd(*ctx.CommentAddCard, *ctx.CommentAddBody, *ctx.CommentAddBoard, *ctx.CommentAddGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.CommentEditUsed:
		runCommentEdit(*ctx.CommentEditID, *ctx.CommentEditBody, *ctx.CommentEditBoard, *ctx.CommentEditGlobal, *ctx.NonInteractive)

	case *ctx.CommentDeleteUsed:
		runCommentDelete(*ctx.CommentDeleteID, *ctx.CommentDeleteBoard, *ctx.CommentDeleteGlobal, *ctx.NonInteractive)

	case *ctx.DoctorUsed:
		runDoctor(*ctx.DoctorBoard, *ctx.DoctorFix, *ctx.DoctorDryRun, *ctx.Json)

	case *ctx.CommitUsed:
		runCommit(*ctx.CommitMessage)

	case *ctx.GlobalSetUsed:
		runGlobalSet(*ctx.GlobalSetBoard, *ctx.NonInteractive)

	case *ctx.GlobalShowUsed:
		runGlobalShow()

	case *ctx.GlobalUnsetUsed:
		runGlobalUnset()

	case *ctx.CompletionUsed:
		runCompletion(*ctx.CompletionShell, ctx.RootCmd)
	}
}
