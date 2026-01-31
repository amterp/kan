package cli

import (
	"fmt"
	"os"

	"github.com/amterp/kan/internal/service"
	"github.com/amterp/ra"
)

func registerAdd(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("add")
	cmd.SetDescription("Add a new card")

	ctx.AddTitle, _ = ra.NewString("title").
		SetUsage("Card title").
		Register(cmd)

	ctx.AddDescription, _ = ra.NewString("description").
		SetOptional(true).
		SetUsage("Card description").
		Register(cmd)

	ctx.AddBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target board").
		Register(cmd)

	ctx.AddColumn, _ = ra.NewString("column").
		SetShort("c").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target column").
		Register(cmd)

	ctx.AddParent, _ = ra.NewString("parent").
		SetShort("p").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Parent card ID or alias").
		Register(cmd)

	ctx.AddFields, _ = ra.NewStringSlice("field").
		SetShort("f").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Set custom field (key=value format, repeatable)").
		Register(cmd)

	ctx.AddUsed, _ = parent.RegisterCmd(cmd)
}

func runAdd(title, description, board, column string, parentCard string, fields []string, nonInteractive, jsonOutput bool) {
	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	// Resolve board
	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	// Validate parent card if provided
	if parentCard != "" {
		_, err := app.CardResolver.Resolve(boardName, parentCard)
		if err != nil {
			Fatal(fmt.Errorf("parent card not found: %s", parentCard))
		}
	}

	// Parse custom fields
	customFields, err := parseCustomFields(fields)
	if err != nil {
		Fatal(err)
	}

	creatorName, err := app.GetAuthor()
	if err != nil {
		Fatal(err)
	}

	input := service.AddCardInput{
		BoardName:    boardName,
		Title:        title,
		Description:  description,
		Column:       column,
		Parent:       parentCard,
		Creator:      creatorName,
		CustomFields: customFields,
	}

	card, hookResults, err := app.CardService.Add(input)
	if err != nil {
		Fatal(err)
	}

	// Get column from board config for output
	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}
	card.Column = boardCfg.GetCardColumn(card.ID)

	if jsonOutput {
		if err := printJson(NewAddOutput(card, hookResults)); err != nil {
			Fatal(err)
		}
		return
	}

	fmt.Printf("Created card %s (%s)\n", card.ID, card.Alias)

	// Display hook results
	for _, result := range hookResults {
		if result.Success {
			if result.Stdout != "" {
				// Show hook output with clear prefix
				fmt.Printf("[hook: %s] %s\n", result.HookName, result.Stdout)
			} else {
				// Show feedback even for quiet successful hooks
				fmt.Printf("[hook: %s] completed\n", result.HookName)
			}
		} else {
			// Show warning with actionable details
			fmt.Fprintf(os.Stderr, "Warning: hook '%s' failed", result.HookName)
			if result.ExitCode > 0 {
				fmt.Fprintf(os.Stderr, " (exit code %d)", result.ExitCode)
			}
			fmt.Fprintln(os.Stderr)
			if result.Stderr != "" {
				fmt.Fprintf(os.Stderr, "  stderr: %s\n", result.Stderr)
			}
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "  error: %v\n", result.Error)
			}
		}
	}
}
