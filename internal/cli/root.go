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
	AddBlockedBy   *[]string
	AddPosition    *int
	AddTop         *bool
	AddBottom      *bool
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

	LinkUsed      *bool
	LinkCard      *string
	LinkBoard     *string
	LinkGlobal    *bool
	LinkBlockedBy *[]string
	LinkRelated   *[]string
	UnlinkUsed    *bool
	UnlinkCard    *string
	UnlinkBoard   *string
	UnlinkGlobal  *bool
	UnlinkRefs    *[]string

	// history command
	HistoryUsed   *bool
	HistoryCard   *string
	HistoryBoard  *string
	HistoryGlobal *bool

	// list command
	ListUsed       *bool
	ListBoard      *string
	ListColumn     *string
	ListGlobal     *bool
	ListSort       *string
	ListDescending *bool
	ListFilter     *[]string
	ListExclude    *[]string
	ListCategory   *string
	ListAll        *bool

	// ready command
	ReadyUsed      *bool
	ReadyBoard     *string
	ReadyExclude   *[]string
	ReadyNoExclude *bool
	ReadyLimit     *int
	ReadyGlobal    *bool
	ReadyAll       *bool

	// edit command
	EditUsed        *bool
	EditCard        *string
	EditBoard       *string
	EditTitle       *string
	EditDescription *string
	EditColumn      *string
	EditParent      *string
	EditPosition    *int
	EditTop         *bool
	EditBottom      *bool
	EditBefore      *string
	EditAfter       *string
	EditAlias       *string
	EditFields      *[]string
	EditStrict      *bool
	EditGlobal      *bool

	// move command
	MoveUsed     *bool
	MoveCard     *string
	MoveColumn   *string
	MoveBoard    *string
	MovePosition *int
	MoveTop      *bool
	MoveBottom   *bool
	MoveBefore   *string
	MoveAfter    *string
	MoveGlobal   *bool

	// serve command
	ServeUsed   *bool
	ServePort   *int
	ServeNoOpen *bool

	// migrate command
	MigrateUsed   *bool
	MigrateDryRun *bool
	MigrateAll    *bool

	// column command
	ColumnUsed *bool

	// column add
	ColumnAddUsed           *bool
	ColumnAddName           *string
	ColumnAddColor          *string
	ColumnAddDescription    *string
	ColumnAddPosition       *int
	ColumnAddLimit          *int
	ColumnAddOnMovePosition *string
	ColumnAddCategory       *string
	ColumnAddBoard          *string

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
	ColumnEditUsed           *bool
	ColumnEditName           *string
	ColumnEditColor          *string
	ColumnEditDescription    *string
	ColumnEditLimit          *int
	ColumnEditOnMovePosition *string
	ColumnEditCategory       *string
	ColumnEditBoard          *string

	// column list
	ColumnListUsed  *bool
	ColumnListBoard *string

	// column categorize
	ColumnCategorizeUsed  *bool
	ColumnCategorizeApply *bool
	ColumnCategorizeBoard *string

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
	DoctorAll    *bool
	DoctorGlobal *bool

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

	// registry command
	RegistryUsed        *bool
	RegistryListUsed    *bool
	RegistryListHidden  *bool
	RegistryHideUsed    *bool
	RegistryHidePath    *string
	RegistryUnhideUsed  *bool
	RegistryUnhidePath  *string
	RegistryRemoveUsed  *bool
	RegistryRemovePath  *string
	RegistryPruneUsed   *bool
	RegistryPruneDryRun *bool

	// docs command
	DocsUsed  *bool
	DocsTopic *string
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
	registerLink(cmd, ctx)
	registerUnlink(cmd, ctx)
	registerDelete(cmd, ctx)
	registerShow(cmd, ctx)
	registerHistory(cmd, ctx)
	registerList(cmd, ctx)
	registerReady(cmd, ctx)
	registerEdit(cmd, ctx)
	registerMove(cmd, ctx)
	registerServe(cmd, ctx)
	registerMigrate(cmd, ctx)
	registerDoctor(cmd, ctx)
	registerCommit(cmd, ctx)
	registerGlobal(cmd, ctx)
	registerRegistry(cmd, ctx)
	registerDocs(cmd, ctx)
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
		case *ctx.RegistryHideUsed:
			unsupportedCommand = "registry hide"
		case *ctx.RegistryUnhideUsed:
			unsupportedCommand = "registry unhide"
		case *ctx.RegistryRemoveUsed:
			unsupportedCommand = "registry remove"
		case *ctx.RegistryPruneUsed:
			unsupportedCommand = "registry prune"
		case *ctx.DocsUsed && *ctx.DocsTopic != "":
			// Bare 'docs' emits a JSON index; a page is markdown either way.
			unsupportedCommand = "docs <topic>"
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
			placementFrom(ctx, ctx.AddPosition, ctx.AddTop, ctx.AddBottom, ctx.AddBefore, ctx.AddAfter),
			*ctx.AddFields, *ctx.AddBlockedBy, *ctx.AddStrict, *ctx.AddGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.LinkUsed:
		runLink(*ctx.LinkCard, *ctx.LinkBoard, *ctx.LinkBlockedBy, *ctx.LinkRelated, *ctx.LinkGlobal, *ctx.NonInteractive, *ctx.Json)
	case *ctx.UnlinkUsed:
		runUnlink(*ctx.UnlinkCard, *ctx.UnlinkBoard, *ctx.UnlinkRefs, *ctx.UnlinkGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.DeleteUsed:
		runDelete(*ctx.DeleteCard, *ctx.DeleteBoard, *ctx.DeleteGlobal, *ctx.NonInteractive)

	case *ctx.ShowUsed:
		runShow(*ctx.ShowCard, *ctx.ShowBoard, *ctx.ShowGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.HistoryUsed:
		runHistory(*ctx.HistoryCard, *ctx.HistoryBoard, *ctx.HistoryGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ListUsed:
		runList(*ctx.ListBoard, *ctx.ListColumn, *ctx.ListSort, *ctx.ListFilter, *ctx.ListExclude, *ctx.ListCategory, *ctx.ListGlobal, *ctx.ListAll, *ctx.ListDescending, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ReadyUsed:
		runReady(*ctx.ReadyBoard, *ctx.ReadyExclude, *ctx.ReadyNoExclude, *ctx.ReadyLimit, *ctx.ReadyGlobal, *ctx.ReadyAll, *ctx.NonInteractive, *ctx.Json)

	case *ctx.EditUsed:
		runEdit(*ctx.EditCard, *ctx.EditBoard, *ctx.EditTitle, *ctx.EditDescription,
			*ctx.EditColumn, *ctx.EditParent, *ctx.EditAlias,
			placementFrom(ctx, ctx.EditPosition, ctx.EditTop, ctx.EditBottom, ctx.EditBefore, ctx.EditAfter),
			*ctx.EditFields, *ctx.EditStrict, *ctx.EditGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.MoveUsed:
		runMove(*ctx.MoveCard, *ctx.MoveColumn, *ctx.MoveBoard,
			placementFrom(ctx, ctx.MovePosition, ctx.MoveTop, ctx.MoveBottom, ctx.MoveBefore, ctx.MoveAfter),
			*ctx.MoveGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ServeUsed:
		runServe(*ctx.ServePort, ctx.RootCmd.Configured("port"), *ctx.ServeNoOpen)

	case *ctx.MigrateUsed:
		if *ctx.MigrateAll {
			runMigrateAll(*ctx.MigrateDryRun, *ctx.NonInteractive)
		} else {
			runMigrate(*ctx.MigrateDryRun)
		}

	case *ctx.ColumnAddUsed:
		runColumnAdd(*ctx.ColumnAddName, *ctx.ColumnAddColor, *ctx.ColumnAddDescription, *ctx.ColumnAddPosition, *ctx.ColumnAddLimit, *ctx.ColumnAddOnMovePosition, *ctx.ColumnAddCategory, *ctx.ColumnAddBoard, *ctx.NonInteractive)

	case *ctx.ColumnDeleteUsed:
		runColumnDelete(*ctx.ColumnDeleteName, *ctx.ColumnDeleteBoard, *ctx.NonInteractive)

	case *ctx.ColumnRenameUsed:
		runColumnRename(*ctx.ColumnRenameOld, *ctx.ColumnRenameNew, *ctx.ColumnRenameBoard, *ctx.NonInteractive)

	case *ctx.ColumnEditUsed:
		runColumnEdit(*ctx.ColumnEditName, *ctx.ColumnEditColor, *ctx.ColumnEditDescription, *ctx.ColumnEditLimit, *ctx.ColumnEditOnMovePosition, *ctx.ColumnEditCategory, ctx.RootCmd.Configured("category"), *ctx.ColumnEditBoard, *ctx.NonInteractive)

	case *ctx.ColumnListUsed:
		runColumnList(*ctx.ColumnListBoard, *ctx.NonInteractive, *ctx.Json)

	case *ctx.ColumnCategorizeUsed:
		runColumnCategorize(*ctx.ColumnCategorizeBoard, *ctx.ColumnCategorizeApply, *ctx.NonInteractive, *ctx.Json)
	case *ctx.ColumnMoveUsed:
		runColumnMove(*ctx.ColumnMoveName, *ctx.ColumnMoveBoard, *ctx.ColumnMovePosition, *ctx.ColumnMoveAfter, *ctx.NonInteractive)

	case *ctx.CommentAddUsed:
		runCommentAdd(*ctx.CommentAddCard, *ctx.CommentAddBody, *ctx.CommentAddBoard, *ctx.CommentAddGlobal, *ctx.NonInteractive, *ctx.Json)

	case *ctx.CommentEditUsed:
		runCommentEdit(*ctx.CommentEditID, *ctx.CommentEditBody, *ctx.CommentEditBoard, *ctx.CommentEditGlobal, *ctx.NonInteractive)

	case *ctx.CommentDeleteUsed:
		runCommentDelete(*ctx.CommentDeleteID, *ctx.CommentDeleteBoard, *ctx.CommentDeleteGlobal, *ctx.NonInteractive)

	case *ctx.DoctorUsed:
		runDoctor(*ctx.DoctorBoard, *ctx.DoctorFix, *ctx.DoctorDryRun, *ctx.Json, *ctx.DoctorAll, *ctx.DoctorGlobal)

	case *ctx.CommitUsed:
		runCommit(*ctx.CommitMessage)

	case *ctx.GlobalSetUsed:
		runGlobalSet(*ctx.GlobalSetBoard, *ctx.NonInteractive)

	case *ctx.GlobalShowUsed:
		runGlobalShow()

	case *ctx.GlobalUnsetUsed:
		runGlobalUnset()

	case *ctx.RegistryListUsed:
		runRegistryList(*ctx.RegistryListHidden, *ctx.Json)

	case *ctx.RegistryHideUsed:
		runRegistrySetHidden(*ctx.RegistryHidePath, true)

	case *ctx.RegistryUnhideUsed:
		runRegistrySetHidden(*ctx.RegistryUnhidePath, false)

	case *ctx.RegistryRemoveUsed:
		runRegistryRemove(*ctx.RegistryRemovePath)

	case *ctx.RegistryPruneUsed:
		runRegistryPrune(*ctx.RegistryPruneDryRun)

	case *ctx.DocsUsed:
		runDocs(*ctx.DocsTopic, *ctx.Json)

	case *ctx.CompletionUsed:
		runCompletion(*ctx.CompletionShell, ctx.RootCmd)
	}
}
