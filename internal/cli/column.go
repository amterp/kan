package cli

import (
	"fmt"

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

	ctx.ColumnAddBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board").
		Register(addCmd)

	ctx.ColumnAddUsed, _ = cmd.RegisterCmd(addCmd)

	// column delete
	deleteCmd := ra.NewCmd("delete")
	deleteCmd.SetDescription("Delete a column and its cards")

	ctx.ColumnDeleteName, _ = ra.NewString("name").
		SetUsage("Name of the column to delete").
		Register(deleteCmd)

	ctx.ColumnDeleteForce, _ = ra.NewBool("force").
		SetShort("f").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Skip confirmation when column has cards").
		Register(deleteCmd)

	ctx.ColumnDeleteBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board").
		Register(deleteCmd)

	ctx.ColumnDeleteUsed, _ = cmd.RegisterCmd(deleteCmd)

	// column rename
	renameCmd := ra.NewCmd("rename")
	renameCmd.SetDescription("Rename a column")

	ctx.ColumnRenameOld, _ = ra.NewString("old").
		SetUsage("Current column name").
		Register(renameCmd)

	ctx.ColumnRenameNew, _ = ra.NewString("new").
		SetUsage("New column name").
		Register(renameCmd)

	ctx.ColumnRenameBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board").
		Register(renameCmd)

	ctx.ColumnRenameUsed, _ = cmd.RegisterCmd(renameCmd)

	// column edit
	editCmd := ra.NewCmd("edit")
	editCmd.SetDescription("Edit column properties")

	ctx.ColumnEditName, _ = ra.NewString("name").
		SetUsage("Name of the column to edit").
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

	ctx.ColumnEditBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board").
		Register(editCmd)

	ctx.ColumnEditUsed, _ = cmd.RegisterCmd(editCmd)

	// column list
	listCmd := ra.NewCmd("list")
	listCmd.SetDescription("List all columns")

	ctx.ColumnListBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board").
		Register(listCmd)

	ctx.ColumnListUsed, _ = cmd.RegisterCmd(listCmd)

	// column move
	moveCmd := ra.NewCmd("move")
	moveCmd.SetDescription("Reorder a column")

	ctx.ColumnMoveName, _ = ra.NewString("name").
		SetUsage("Name of the column to move").
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
		Register(moveCmd)

	ctx.ColumnMoveBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board").
		Register(moveCmd)

	ctx.ColumnMoveUsed, _ = cmd.RegisterCmd(moveCmd)

	ctx.ColumnUsed, _ = parent.RegisterCmd(cmd)
}

func runColumnAdd(name, color, description string, position, limit int, board string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
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

	PrintSuccess("Added column %q to board %q", name, boardName)
}

func runColumnDelete(name, board string, force, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	// Check how many cards are in the column
	cardCount, err := app.BoardService.GetColumnCardCount(boardName, name)
	if err != nil {
		Fatal(err)
	}

	// Confirm if there are cards and no --force flag
	if cardCount > 0 && !force {
		if nonInteractive {
			Fatal(fmt.Errorf("column %q has %d cards; use --force to confirm deletion", name, cardCount))
		}

		cardWord := "cards"
		if cardCount == 1 {
			cardWord = "card"
		}
		confirmed, err := app.Prompter.Confirm(
			fmt.Sprintf("Column %q has %d %s. Delete column and all cards?", name, cardCount, cardWord),
			false,
		)
		if err != nil {
			Fatal(err)
		}
		if !confirmed {
			PrintInfo("Cancelled")
			return
		}
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

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.BoardService.RenameColumn(boardName, oldName, newName); err != nil {
		Fatal(err)
	}

	PrintSuccess("Renamed column %q to %q in board %q", oldName, newName, boardName)
}

func runColumnEdit(name, color, description string, limit int, board string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if color == "" && description == "" && limit < 0 {
		Fatal(fmt.Errorf("no changes specified; use --color, --description, or --limit"))
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

	PrintSuccess("Updated column %q in board %q", name, boardName)
}

func runColumnList(board string, nonInteractive, jsonOutput bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		columns := make([]ColumnInfo, len(boardCfg.Columns))
		for i, col := range boardCfg.Columns {
			columns[i] = ColumnInfo{
				Name:        col.Name,
				Color:       col.Color,
				Description: col.Description,
				Limit:    col.Limit,
				CardCount:   len(col.CardIDs),
			}
		}
		if err := printJson(NewColumnsOutput(columns)); err != nil {
			Fatal(err)
		}
		return
	}

	if len(boardCfg.Columns) == 0 {
		PrintInfo("No columns found")
		return
	}

	for _, col := range boardCfg.Columns {
		cardWord := "cards"
		if len(col.CardIDs) == 1 {
			cardWord = "card"
		}
		swatch := ColorSwatch(col.Color)
		var count string
		if col.Limit > 0 {
			count = RenderMuted(fmt.Sprintf("(%d/%d %s)", len(col.CardIDs), col.Limit, cardWord))
		} else {
			count = RenderMuted(fmt.Sprintf("(%d %s)", len(col.CardIDs), cardWord))
		}
		fmt.Printf("%-15s %s %s\n", col.Name, swatch, count)
		if col.Description != "" {
			fmt.Printf("  %s\n", RenderMuted(col.Description))
		}
	}
}

func runColumnMove(name, board string, position int, after string, nonInteractive bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
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
