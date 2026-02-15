package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/ra"
)

func registerBoard(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("board")
	cmd.SetDescription("Manage boards")

	// board create
	createCmd := ra.NewCmd("create")
	createCmd.SetDescription("Create a new board")

	ctx.BoardCreateName, _ = ra.NewString("name").
		SetUsage("Name of the board to create").
		Register(createCmd)

	ctx.BoardCreateUsed, _ = cmd.RegisterCmd(createCmd)

	// board describe
	describeCmd := ra.NewCmd("describe")
	describeCmd.SetDescription("Show board documentation (columns, fields, settings)")

	ctx.BoardDescribeName, _ = ra.NewString("name").
		SetOptional(true).
		SetUsage("Board name (defaults to resolved board)").
		SetCompletionFunc(completeBoards).
		Register(describeCmd)

	ctx.BoardDescribeBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board").
		SetCompletionFunc(completeBoards).
		Register(describeCmd)

	ctx.BoardDescribeUsed, _ = cmd.RegisterCmd(describeCmd)

	// board list
	listCmd := ra.NewCmd("list")
	listCmd.SetDescription("List all boards")

	ctx.BoardListUsed, _ = cmd.RegisterCmd(listCmd)

	ctx.BoardUsed, _ = parent.RegisterCmd(cmd)
}

func runBoardCreate(name string) {
	app, err := NewApp(true)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	if err := app.BoardService.Create(name); err != nil {
		Fatal(err)
	}

	PrintSuccess("Created board %q", name)
}

func runBoardList(jsonOutput bool) {
	app, err := NewApp(true)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	boards, err := app.BoardService.List()
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		if err := printJson(NewBoardsOutput(boards)); err != nil {
			Fatal(err)
		}
		return
	}

	if len(boards) == 0 {
		PrintInfo("No boards found")
		return
	}

	fmt.Println(RenderMuted("Boards:"))
	for _, board := range boards {
		fmt.Printf("  %s %s\n", RenderMuted("•"), board)
	}
}

func runBoardDescribe(name, board string, nonInteractive, jsonOutput bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	// Use positional name if provided, otherwise fall back to --board flag
	boardArg := name
	if boardArg == "" {
		boardArg = board
	}

	boardName, err := app.BoardResolver.Resolve(boardArg, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		printBoardDescribeJson(boardCfg)
		return
	}

	printBoardDescribeHuman(boardCfg)
}

func printBoardDescribeJson(cfg *model.BoardConfig) {
	columns := make([]BoardDescribeColumnInfo, len(cfg.Columns))
	for i, col := range cfg.Columns {
		columns[i] = BoardDescribeColumnInfo{
			Name:        col.Name,
			Color:       col.Color,
			Description: col.Description,
			Limit:       col.Limit,
			CardCount:   len(col.CardIDs),
			IsDefault:   col.Name == cfg.DefaultColumn,
		}
	}

	output := BoardDescribeOutput{
		Board: BoardDescribeInfo{
			Name:          cfg.Name,
			Schema:        cfg.KanSchema,
			DefaultColumn: cfg.DefaultColumn,
			Columns:       columns,
			CustomFields:  cfg.CustomFields,
			CardDisplay:   cfg.CardDisplay,
			LinkRules:     cfg.LinkRules,
			PatternHooks:  cfg.PatternHooks,
		},
	}

	if err := printJson(output); err != nil {
		Fatal(err)
	}
}

func printBoardDescribeHuman(cfg *model.BoardConfig) {
	// Header
	fmt.Printf("Board: %s\n", cfg.Name)
	fmt.Printf("Schema: %s\n", cfg.KanSchema)

	// Columns
	fmt.Println()
	fmt.Println("Columns:")
	for _, col := range cfg.Columns {
		cardWord := "cards"
		if len(col.CardIDs) == 1 {
			cardWord = "card"
		}
		swatch := ColorSwatch(col.Color)
		defaultTag := ""
		if col.Name == cfg.DefaultColumn {
			defaultTag = ", default"
		}
		limitStr := ""
		if col.Limit > 0 {
			limitStr = fmt.Sprintf("/%d", col.Limit)
		}
		count := RenderMuted(fmt.Sprintf("(%d%s %s%s)", len(col.CardIDs), limitStr, cardWord, defaultTag))
		fmt.Printf("  %-17s %s %s\n", col.Name, swatch, count)
		if col.Description != "" {
			fmt.Printf("    %s\n", col.Description)
		}
	}

	// Custom Fields
	if len(cfg.CustomFields) > 0 {
		fmt.Println()
		fmt.Println("Custom Fields:")

		// Sort field names for stable output
		fieldNames := make([]string, 0, len(cfg.CustomFields))
		for name := range cfg.CustomFields {
			fieldNames = append(fieldNames, name)
		}
		sort.Strings(fieldNames)

		for _, name := range fieldNames {
			schema := cfg.CustomFields[name]
			attrs := []string{schema.Type}
			if schema.Wanted {
				attrs = append(attrs, "wanted")
			}
			fmt.Printf("  %s %s\n", name, RenderMuted("("+strings.Join(attrs, ", ")+")"))
			if schema.Description != "" {
				fmt.Printf("    %s\n", schema.Description)
			}
			if len(schema.Options) > 0 {
				printFieldOptions(schema.Options)
			}
		}
	}

	// Card Display
	cd := cfg.CardDisplay
	if cd.TypeIndicator != "" || len(cd.Badges) > 0 || len(cd.Metadata) > 0 {
		fmt.Println()
		fmt.Println("Card Display:")
		if cd.TypeIndicator != "" {
			fmt.Printf("  Type indicator: %s\n", cd.TypeIndicator)
		}
		if len(cd.Badges) > 0 {
			fmt.Printf("  Badges: %s\n", strings.Join(cd.Badges, ", "))
		}
		if len(cd.Metadata) > 0 {
			fmt.Printf("  Metadata: %s\n", strings.Join(cd.Metadata, ", "))
		}
	}

	// Link Rules
	if len(cfg.LinkRules) > 0 {
		fmt.Println()
		fmt.Println("Link Rules:")
		for _, rule := range cfg.LinkRules {
			fmt.Printf("  %s  %s\n", rule.Name, RenderMuted(rule.Pattern))
		}
	}

	// Pattern Hooks
	if len(cfg.PatternHooks) > 0 {
		fmt.Println()
		fmt.Println("Pattern Hooks:")
		for _, hook := range cfg.PatternHooks {
			fmt.Printf("  %s  %s\n", hook.Name, RenderMuted(hook.PatternTitle))
		}
	}
}

// printFieldOptions renders option lists for custom fields.
// Uses expanded multi-line format when any option has a description,
// compact single-line format otherwise.
func printFieldOptions(options []model.CustomFieldOption) {
	hasDescriptions := false
	for _, opt := range options {
		if opt.Description != "" {
			hasDescriptions = true
			break
		}
	}

	if hasDescriptions {
		fmt.Println("    Options:")
		for _, opt := range options {
			if opt.Description != "" {
				fmt.Printf("      %-12s - %s\n", opt.Value, opt.Description)
			} else {
				fmt.Printf("      %s\n", opt.Value)
			}
		}
	} else {
		values := make([]string, len(options))
		for i, opt := range options {
			values[i] = opt.Value
		}
		fmt.Printf("    Options: %s\n", strings.Join(values, ", "))
	}
}
