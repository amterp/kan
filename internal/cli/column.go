package cli

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/ra"
)

func registerColumn(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("column")
	cmd.SetDescription("Manage columns")

	// column add
	addCmd := ra.NewCmd("add")
	addCmd.SetDescription("Add a new column")

	ctx.ColumnAddName, _ = ra.NewString("name").
		SetUsage("Name of the column to create (lowercase alphanumeric with hyphens)").
		Register(addCmd)

	ctx.ColumnAddColor, _ = ra.NewString("color").
		SetShort("C").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Hex color (e.g., '#9333ea'). Auto-assigned if not specified.").
		Register(addCmd)

	ctx.ColumnAddPosition, _ = ra.NewInt("position").
		SetShort("p").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Insert position (0-indexed). Appends to end if not specified.").
		Register(addCmd)

	ctx.ColumnAddDescription, _ = ra.NewString("description").
		SetShort("d").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Description of the column's purpose").
		Register(addCmd)

	ctx.ColumnAddLimit, _ = ra.NewInt("limit").
		SetShort("l").
		SetOptional(true).
		SetFlagOnly(true).
		SetDefault(-1).
		SetUsage("Maximum number of cards allowed in this column (0 = no limit)").
		Register(addCmd)

	ctx.ColumnAddOnMovePosition, _ = ra.NewString("on-move-default-position").
		SetOptional(true).
		SetFlagOnly(true).
		SetEnumConstraint(model.ValidColumnPositions).
		SetUsage("Where a card moved into this column lands: 'top' (default) or 'bottom'").
		Register(addCmd)

	ctx.ColumnAddCategory, _ = ra.NewString("category").
		SetOptional(true).
		SetFlagOnly(true).
		SetEnumConstraint(model.ValidCategories).
		SetUsage("What kind of work the column holds, read by queries across columns and boards").
		Register(addCmd)

	ctx.ColumnAddBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board (name or ID)").
		SetCompletionFunc(completeBoards).
		Register(addCmd)

	ctx.ColumnAddUsed, _ = cmd.RegisterCmd(addCmd)

	// column delete
	deleteCmd := ra.NewCmd("delete")
	deleteCmd.SetDescription("Delete a column and its cards")

	ctx.ColumnDeleteName, _ = ra.NewString("name").
		SetUsage("Name of the column to delete").
		SetCompletionFunc(completeColumns).
		Register(deleteCmd)

	ctx.ColumnDeleteBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board (name or ID)").
		SetCompletionFunc(completeBoards).
		Register(deleteCmd)

	ctx.ColumnDeleteUsed, _ = cmd.RegisterCmd(deleteCmd)

	// column rename
	renameCmd := ra.NewCmd("rename")
	renameCmd.SetDescription("Rename a column")

	ctx.ColumnRenameOld, _ = ra.NewString("old").
		SetUsage("Current column name").
		SetCompletionFunc(completeColumns).
		Register(renameCmd)

	ctx.ColumnRenameNew, _ = ra.NewString("new").
		SetUsage("New column name").
		Register(renameCmd)

	ctx.ColumnRenameBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board (name or ID)").
		SetCompletionFunc(completeBoards).
		Register(renameCmd)

	ctx.ColumnRenameUsed, _ = cmd.RegisterCmd(renameCmd)

	// column edit
	editCmd := ra.NewCmd("edit")
	editCmd.SetDescription("Edit column properties")

	ctx.ColumnEditName, _ = ra.NewString("name").
		SetUsage("Name of the column to edit").
		SetCompletionFunc(completeColumns).
		Register(editCmd)

	ctx.ColumnEditColor, _ = ra.NewString("color").
		SetShort("C").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("New hex color (e.g., '#9333ea')").
		Register(editCmd)

	ctx.ColumnEditDescription, _ = ra.NewString("description").
		SetShort("d").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("New description for the column").
		Register(editCmd)

	ctx.ColumnEditLimit, _ = ra.NewInt("limit").
		SetShort("l").
		SetOptional(true).
		SetFlagOnly(true).
		SetDefault(-1).
		SetUsage("Column limit (0 = clear limit, >0 = set limit)").
		Register(editCmd)

	ctx.ColumnEditOnMovePosition, _ = ra.NewString("on-move-default-position").
		SetOptional(true).
		SetFlagOnly(true).
		SetEnumConstraint(model.ValidColumnPositions).
		SetUsage("Where a card moved into this column lands: 'top' (default) or 'bottom'").
		Register(editCmd)

	// No enum constraint: an empty value clears the category, and ra would reject it.
	ctx.ColumnEditCategory, _ = ra.NewString("category").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("What kind of work the column holds: " + model.ValidCategoriesList() + ". Empty clears it").
		SetCompletionFunc(completeCategories).
		Register(editCmd)

	ctx.ColumnEditBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board (name or ID)").
		SetCompletionFunc(completeBoards).
		Register(editCmd)

	ctx.ColumnEditUsed, _ = cmd.RegisterCmd(editCmd)

	// column list
	listCmd := ra.NewCmd("list")
	listCmd.SetDescription("List all columns")

	ctx.ColumnListBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board (name or ID)").
		SetCompletionFunc(completeBoards).
		Register(listCmd)

	ctx.ColumnListUsed, _ = cmd.RegisterCmd(listCmd)

	// column move
	moveCmd := ra.NewCmd("move")
	moveCmd.SetDescription("Reorder a column")

	ctx.ColumnMoveName, _ = ra.NewString("name").
		SetUsage("Name of the column to move").
		SetCompletionFunc(completeColumns).
		Register(moveCmd)

	ctx.ColumnMovePosition, _ = ra.NewInt("position").
		SetShort("p").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target position (0-indexed)").
		Register(moveCmd)

	ctx.ColumnMoveAfter, _ = ra.NewString("after").
		SetShort("a").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Insert after this column").
		SetCompletionFunc(completeColumns).
		Register(moveCmd)

	ctx.ColumnMoveBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board (name or ID)").
		SetCompletionFunc(completeBoards).
		Register(moveCmd)

	ctx.ColumnMoveUsed, _ = cmd.RegisterCmd(moveCmd)

	// column categorize
	categorizeCmd := ra.NewCmd("categorize")
	categorizeCmd.SetDescription("Suggest categories for uncategorized columns from their names")

	ctx.ColumnCategorizeApply, _ = ra.NewBool("apply").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Write the suggested categories instead of only printing them").
		Register(categorizeCmd)

	ctx.ColumnCategorizeBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board (name or ID)").
		SetCompletionFunc(completeBoards).
		Register(categorizeCmd)

	ctx.ColumnCategorizeUsed, _ = cmd.RegisterCmd(categorizeCmd)

	ctx.ColumnUsed, _ = parent.RegisterCmd(cmd)
}

func runColumnAdd(name, color, description string, position, limit int, onMovePosition, category, board string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	_, boardName, err := app.ResolveBoard(board, !nonInteractive, forWrite)
	if err != nil {
		Fatal(err)
	}

	// Position of -1 means append to end (since 0 is a valid position)
	pos := -1
	if position > 0 {
		pos = position
	}

	if err := app.BoardService.AddColumn(boardName, name, color, description, pos); err != nil {
		Fatal(err)
	}

	// limit: -1 = not specified, 0 = clear limit, >0 = set limit
	if limit >= 0 {
		if err := app.BoardService.UpdateColumnLimit(boardName, name, limit); err != nil {
			Fatal(err)
		}
	}

	if onMovePosition != "" {
		if err := app.BoardService.UpdateColumnOnMoveDefaultPosition(boardName, name, onMovePosition); err != nil {
			Fatal(err)
		}
	}

	if category != "" {
		if err := app.BoardService.UpdateColumnCategory(boardName, name, category); err != nil {
			Fatal(err)
		}
	}

	PrintSuccess("Added column %q to board %q", name, boardName)
}

func runColumnDelete(name, board string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	_, boardName, err := app.ResolveBoard(board, !nonInteractive, forWrite)
	if err != nil {
		Fatal(err)
	}

	deletedCards, err := app.BoardService.DeleteColumn(boardName, name)
	if err != nil {
		Fatal(err)
	}

	if deletedCards > 0 {
		cardWord := "cards"
		if deletedCards == 1 {
			cardWord = "card"
		}
		PrintSuccess("Deleted column %q and %d %s from board %q", name, deletedCards, cardWord, boardName)
	} else {
		PrintSuccess("Deleted column %q from board %q", name, boardName)
	}
}

func runColumnRename(oldName, newName, board string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	_, boardName, err := app.ResolveBoard(board, !nonInteractive, forWrite)
	if err != nil {
		Fatal(err)
	}

	if err := app.BoardService.RenameColumn(boardName, oldName, newName); err != nil {
		Fatal(err)
	}

	PrintSuccess("Renamed column %q to %q in board %q", oldName, newName, boardName)
}

func runColumnEdit(name, color, description string, limit int, onMovePosition, category string, categorySet bool, board string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	_, boardName, err := app.ResolveBoard(board, !nonInteractive, forWrite)
	if err != nil {
		Fatal(err)
	}

	if color == "" && description == "" && limit < 0 && onMovePosition == "" && !categorySet {
		Fatal(fmt.Errorf("no changes specified; use --color, --description, --limit, " +
			"--on-move-default-position, or --category"))
	}

	// First, so an invalid category fails before any other change is written.
	if categorySet {
		if err := app.BoardService.UpdateColumnCategory(boardName, name, category); err != nil {
			Fatal(err)
		}
	}

	if color != "" {
		if err := app.BoardService.UpdateColumnColor(boardName, name, color); err != nil {
			Fatal(err)
		}
	}

	if description != "" {
		if err := app.BoardService.UpdateColumnDescription(boardName, name, description); err != nil {
			Fatal(err)
		}
	}

	// limit: -1 = not specified, 0 = clear limit, >0 = set limit
	if limit >= 0 {
		if err := app.BoardService.UpdateColumnLimit(boardName, name, limit); err != nil {
			Fatal(err)
		}
	}

	if onMovePosition != "" {
		if err := app.BoardService.UpdateColumnOnMoveDefaultPosition(boardName, name, onMovePosition); err != nil {
			Fatal(err)
		}
	}

	PrintSuccess("Updated column %q in board %q", name, boardName)
}

func runColumnList(board string, nonInteractive, jsonOutput bool) {
	app, err := NewAppWithOptions(AppOptions{Interactive: !nonInteractive, ReadOnly: true})
	if err != nil {
		Fatal(err)
	}

	project, boardName, err := app.ResolveBoard(board, !nonInteractive, forRead)
	if err != nil {
		Fatal(err)
	}
	project.PrintTarget(boardName)

	boardCfg, err := project.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	// Build card count per column
	cards, err := project.CardService.List(boardName, "")
	cardCounts := make(map[string]int)
	if err == nil {
		for _, card := range cards {
			cardCounts[card.Column]++
		}
	}

	if jsonOutput {
		columns := make([]ColumnInfo, len(boardCfg.Columns))
		for i, col := range boardCfg.Columns {
			columns[i] = ColumnInfo{
				Name:        col.Name,
				Color:       col.Color,
				Description: col.Description,
				Limit:       col.Limit,
				Category:    col.Category,
				CardCount:   cardCounts[col.Name],
			}
		}
		if err := printJson(NewColumnsOutput(columns, boardCfg.Ready)); err != nil {
			Fatal(err)
		}
		return
	}

	if len(boardCfg.Columns) == 0 {
		PrintInfo("No columns found")
		return
	}

	for _, col := range boardCfg.Columns {
		n := cardCounts[col.Name]
		cardWord := "cards"
		if n == 1 {
			cardWord = "card"
		}
		swatch := ColorSwatch(col.Color)
		var count string
		if col.Limit > 0 {
			count = RenderMuted(fmt.Sprintf("(%d/%d %s)", n, col.Limit, cardWord))
		} else {
			count = RenderMuted(fmt.Sprintf("(%d %s)", n, cardWord))
		}
		fmt.Printf("%-15s %s %s%s\n", col.Name, swatch, count, categorySuffix(col))
		if col.Description != "" {
			fmt.Printf("  %s\n", RenderMuted(col.Description))
		}
	}

	if !boardCfg.Ready.IsEmpty() {
		fmt.Println()
		printReadySettings(boardCfg.Ready, "")
	}
}

// categorySuffix renders a column's category for text output, or nothing
// when the column has none.
func categorySuffix(col model.Column) string {
	if col.Category == "" {
		return ""
	}
	return "  " + col.Category
}

// printReadySettings prints a board's [ready] settings, one line per key that is set.
func printReadySettings(ready model.ReadyConfig, indent string) {
	if len(ready.Order) > 0 {
		fmt.Printf("%sReady order:   %s\n", indent, strings.Join(ready.Order, ", "))
	}
	if len(ready.Exclude) > 0 {
		fmt.Printf("%sReady exclude: %s\n", indent, strings.Join(ready.Exclude, ", "))
	}
}

func runColumnCategorize(board string, apply, nonInteractive, jsonOutput bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	_, boardName, err := app.ResolveBoard(board, !nonInteractive, forWrite)
	if err != nil {
		Fatal(err)
	}

	suggestions, err := app.BoardService.CategorizeColumns(boardName, apply)
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		out := CategorizeOutput{Board: boardName, Applied: apply, Suggestions: []CategorySuggestionInfo{}}
		for _, s := range suggestions {
			out.Suggestions = append(out.Suggestions, CategorySuggestionInfo{Column: s.Column, Category: s.Category})
		}
		if err := printJson(out); err != nil {
			Fatal(err)
		}
		return
	}

	if len(suggestions) == 0 {
		PrintInfo("All columns on board %q have categories", boardName)
		return
	}

	matched := 0
	for _, s := range suggestions {
		if s.Category != "" {
			matched++
		}
	}

	if apply {
		columnWord := "columns"
		if matched == 1 {
			columnWord = "column"
		}
		PrintSuccess("Set categories on %d %s of board %q", matched, columnWord, boardName)
	} else {
		fmt.Printf("Suggested categories for board %q:\n", boardName)
	}
	for _, s := range suggestions {
		category := s.Category
		if category == "" {
			category = RenderMuted("no suggestion")
		}
		fmt.Printf("  %-15s %s\n", s.Column, category)
	}
	if !apply && matched > 0 {
		fmt.Println()
		boardFlag := ""
		if board != "" {
			boardFlag = " -b " + boardName
		}
		fmt.Printf("Run \"kan column categorize%s --apply\" to set them.\n", boardFlag)
	}
	if matched < len(suggestions) {
		fmt.Println()
		fmt.Println(`Set the rest with "kan column edit <column> --category <category>".`)
	}
}

func runColumnMove(name, board string, position int, after string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	_, boardName, err := app.ResolveBoard(board, !nonInteractive, forWrite)
	if err != nil {
		Fatal(err)
	}

	// Determine target position
	targetPos := position
	if after != "" {
		// Find the position of the "after" column and insert after it
		boardCfg, err := app.BoardService.Get(boardName)
		if err != nil {
			Fatal(err)
		}

		afterIdx := boardCfg.GetColumnIndex(after)
		if afterIdx < 0 {
			Fatal(fmt.Errorf("column %q not found in board %q", after, boardName))
		}
		targetPos = afterIdx + 1
	}

	if targetPos < 0 {
		Fatal(fmt.Errorf("must specify --position or --after"))
	}

	if err := app.BoardService.ReorderColumn(boardName, name, targetPos); err != nil {
		Fatal(err)
	}

	PrintSuccess("Moved column %q to position %d in board %q", name, targetPos, boardName)
}
