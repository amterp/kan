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

	// add command
	AddUsed        *bool
	AddTitle       *string
	AddDescription *string
	AddBoard       *string
	AddColumn      *string
	AddParent      *string
	AddFields      *[]string
	AddStrict      *bool

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
	EditParent      *string
	EditAlias       *string
	EditFields      *[]string
	EditStrict      *bool

	// serve command
	ServeUsed   *bool
	ServePort   *int
	ServeNoOpen *bool

	// migrate command
	MigrateUsed   *bool
	MigrateDryRun *bool

	// column command
	ColumnUsed *bool

	// column add
	ColumnAddUsed     *bool
	ColumnAddName     *string
	ColumnAddColor    *string
	ColumnAddPosition *int
	ColumnAddBoard    *string

	// column delete
	ColumnDeleteUsed  *bool
	ColumnDeleteName  *string
	ColumnDeleteForce *bool
	ColumnDeleteBoard *string

	// column rename
	ColumnRenameUsed  *bool
	ColumnRenameOld   *string
	ColumnRenameNew   *string
	ColumnRenameBoard *string

	// column edit
	ColumnEditUsed  *bool
	ColumnEditName  *string
	ColumnEditColor *string
	ColumnEditBoard *string

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
	CommentAddUsed  *bool
	CommentAddCard  *string
	CommentAddBody  *string
	CommentAddBoard *string

	// comment edit
	CommentEditUsed  *bool
	CommentEditID    *string
	CommentEditBody  *string
	CommentEditBoard *string

	// comment delete
	CommentDeleteUsed  *bool
	CommentDeleteID    *string
	CommentDeleteBoard *string

	// doctor command
	DoctorUsed   *bool
	DoctorFix    *bool
	DoctorDryRun *bool
	DoctorBoard  *string
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
	registerShow(cmd, ctx)
	registerList(cmd, ctx)
	registerEdit(cmd, ctx)
	registerServe(cmd, ctx)
	registerMigrate(cmd, ctx)
	registerDoctor(cmd, ctx)

	// Parse command line
	cmd.ParseOrExit(os.Args[1:])

	// Execute the appropriate command
	executeCommand(ctx)
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
		}
		if unsupportedCommand != "" {
			warnJsonNotSupported(unsupportedCommand)
		}
	}

	switch {
	case *ctx.InitUsed:
		runInit(*ctx.InitLocation, *ctx.InitName, *ctx.InitColumns, *ctx.InitProjectName)

	case *ctx.BoardCreateUsed:
		runBoardCreate(*ctx.BoardCreateName)

	case *ctx.BoardListUsed:
		runBoardList(*ctx.Json)

	case *ctx.AddUsed:
		runAdd(*ctx.AddTitle, *ctx.AddDescription, *ctx.AddBoard, *ctx.AddColumn, *ctx.AddParent, *ctx.AddFields, *ctx.AddStrict, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ShowUsed:
		runShow(*ctx.ShowCard, *ctx.ShowBoard, *ctx.Json)

	case *ctx.ListUsed:
		runList(*ctx.ListBoard, *ctx.ListColumn, *ctx.Json)

	case *ctx.EditUsed:
		runEdit(*ctx.EditCard, *ctx.EditBoard, *ctx.EditTitle, *ctx.EditDescription,
			*ctx.EditColumn, *ctx.EditParent, *ctx.EditAlias,
			*ctx.EditFields, *ctx.EditStrict, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ServeUsed:
		runServe(*ctx.ServePort, *ctx.ServeNoOpen)

	case *ctx.MigrateUsed:
		runMigrate(*ctx.MigrateDryRun)

	case *ctx.ColumnAddUsed:
		runColumnAdd(*ctx.ColumnAddName, *ctx.ColumnAddColor, *ctx.ColumnAddPosition, *ctx.ColumnAddBoard, *ctx.NonInteractive)

	case *ctx.ColumnDeleteUsed:
		runColumnDelete(*ctx.ColumnDeleteName, *ctx.ColumnDeleteBoard, *ctx.ColumnDeleteForce, *ctx.NonInteractive)

	case *ctx.ColumnRenameUsed:
		runColumnRename(*ctx.ColumnRenameOld, *ctx.ColumnRenameNew, *ctx.ColumnRenameBoard, *ctx.NonInteractive)

	case *ctx.ColumnEditUsed:
		runColumnEdit(*ctx.ColumnEditName, *ctx.ColumnEditColor, *ctx.ColumnEditBoard, *ctx.NonInteractive)

	case *ctx.ColumnListUsed:
		runColumnList(*ctx.ColumnListBoard, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ColumnMoveUsed:
		runColumnMove(*ctx.ColumnMoveName, *ctx.ColumnMoveBoard, *ctx.ColumnMovePosition, *ctx.ColumnMoveAfter, *ctx.NonInteractive)

	case *ctx.CommentAddUsed:
		runCommentAdd(*ctx.CommentAddCard, *ctx.CommentAddBody, *ctx.CommentAddBoard, *ctx.NonInteractive, *ctx.Json)

	case *ctx.CommentEditUsed:
		runCommentEdit(*ctx.CommentEditID, *ctx.CommentEditBody, *ctx.CommentEditBoard, *ctx.NonInteractive)

	case *ctx.CommentDeleteUsed:
		runCommentDelete(*ctx.CommentDeleteID, *ctx.CommentDeleteBoard, *ctx.NonInteractive)

	case *ctx.DoctorUsed:
		runDoctor(*ctx.DoctorBoard, *ctx.DoctorFix, *ctx.DoctorDryRun, *ctx.Json)
	}
}
