package cli

import (
	"fmt"
	"os"
	"strings"

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

	ctx.AddStrict, _ = ra.NewBool("strict").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Error if wanted fields are missing (default: warn)").
		Register(cmd)

	ctx.AddUsed, _ = parent.RegisterCmd(cmd)
}

func runAdd(title, description, board, column string, parentCard string, fields []string, strict, nonInteractive, jsonOutput bool) {
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

	// Get board config for validation and output
	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	// In strict mode, check wanted fields BEFORE creating the card
	if strict {
		missingWanted := service.CheckWantedFieldsForProposal(nil, customFields, boardCfg)
		if len(missingWanted) > 0 {
			Fatal(fmt.Errorf("card not created: missing wanted fields: %s", formatMissingWantedFields(missingWanted)))
		}
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

	card.Column = boardCfg.GetCardColumn(card.ID)

	// Check for missing wanted fields (for warnings in non-strict mode)
	missingWanted := service.CheckWantedFields(card, boardCfg)

	if jsonOutput {
		if err := printJson(NewAddOutput(card, hookResults)); err != nil {
			Fatal(err)
		}
		return
	}

	PrintSuccess("Created card %s (%s)", RenderID(card.ID), card.Alias)

	// Display hook results
	printHookResults(hookResults)

	// Warn about missing wanted fields
	printMissingWantedWarnings(missingWanted)
}

// printHookResults displays hook results with appropriate styling.
// Silent success is fine - only show output when there's something to report.
func printHookResults(results []*service.HookResult) {
	for _, result := range results {
		if result.Success {
			// Only show output if the hook produced something
			if result.Stdout != "" {
				PrintInfo("hook: %s %s", result.HookName, result.Stdout)
			}
			// Silent success is fine - no news is good news
		} else {
			// Always show failures with actionable details
			msg := fmt.Sprintf("hook '%s' failed", result.HookName)
			if result.ExitCode > 0 {
				msg += fmt.Sprintf(" (exit code %d)", result.ExitCode)
			}
			PrintWarning("%s", msg)
			if result.Stderr != "" {
				fmt.Fprintf(os.Stderr, "  stderr: %s\n", result.Stderr)
			}
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "  error: %v\n", result.Error)
			}
		}
	}
}

// printMissingWantedWarnings displays warnings for missing wanted fields.
func printMissingWantedWarnings(missing []service.MissingWantedField) {
	if len(missing) == 0 {
		return
	}
	PrintWarning("Card is missing wanted fields:")
	for _, mf := range missing {
		if len(mf.Options) > 0 {
			fmt.Fprintf(os.Stderr, "  - %s (%s): valid values are %s\n",
				mf.FieldName, mf.FieldType, strings.Join(mf.Options, ", "))
		} else {
			fmt.Fprintf(os.Stderr, "  - %s (%s)\n", mf.FieldName, mf.FieldType)
		}
	}
}

// formatMissingWantedFields formats missing wanted fields for error messages.
func formatMissingWantedFields(missing []service.MissingWantedField) string {
	names := make([]string, len(missing))
	for i, mf := range missing {
		names[i] = mf.FieldName
	}
	return strings.Join(names, ", ")
}
