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
		SetCompletionFunc(completeBoards).
		Register(cmd)

	ctx.AddColumn, _ = ra.NewString("column").
		SetShort("c").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Target column").
		SetCompletionFunc(completeColumns).
		Register(cmd)

	ctx.AddParent, _ = ra.NewString("parent").
		SetShort("p").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Parent card ID or alias").
		SetCompletionFunc(completeCards).
		Register(cmd)

	ctx.AddPosition, _ = ra.NewInt("position").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Insert at index (0 = top, -1 = end, negatives count from end)").
		SetExcludes([]string{"before", "after"}).
		Register(cmd)

	ctx.AddBefore, _ = ra.NewString("before").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Insert before this card (ID or alias); uses its column if -c omitted").
		SetCompletionFunc(completeCards).
		SetExcludes([]string{"position", "after"}).
		Register(cmd)

	ctx.AddAfter, _ = ra.NewString("after").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Insert after this card (ID or alias); uses its column if -c omitted").
		SetCompletionFunc(completeCards).
		SetExcludes([]string{"position", "before"}).
		Register(cmd)

	ctx.AddFields, _ = ra.NewStringSlice("field").
		SetShort("f").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Set custom field (key=value, repeatable; set fields also accept comma-separated values)").
		Register(cmd)

	ctx.AddStrict, _ = ra.NewBool("strict").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Error if wanted fields are missing (default: warn)").
		Register(cmd)

	ctx.AddUsed, _ = parent.RegisterCmd(cmd)
}

// cardPlacement holds the CLI-level request for where a card should go within a
// column. At most one of (position, before, after) is meaningful; positionSet
// distinguishes an explicit --position 0 from the flag being omitted.
type cardPlacement struct {
	position    int
	positionSet bool
	before      string
	after       string
}

func (p cardPlacement) isSet() bool {
	return p.positionSet || p.before != "" || p.after != ""
}

// resolvePlacement turns a cardPlacement into service-level values: an optional
// explicit index plus canonical before/after anchor IDs (resolved from the
// user's ID/alias input). Column inference from the anchor happens in the service
// layer, so it is intentionally not returned here. excludeID, when set, is the
// card being moved - it may not anchor to itself.
func resolvePlacement(app *App, boardName, excludeID string, p cardPlacement) (position *int, beforeID, afterID string, err error) {
	if p.positionSet {
		pos := p.position
		position = &pos
	}

	anchor, isBefore := p.before, true
	if p.after != "" {
		anchor, isBefore = p.after, false
	}
	if anchor == "" {
		return position, "", "", nil
	}

	card, rerr := app.CardResolver.Resolve(boardName, anchor)
	if rerr != nil {
		return nil, "", "", fmt.Errorf("anchor card not found: %s", anchor)
	}
	if excludeID != "" && card.ID == excludeID {
		return nil, "", "", fmt.Errorf("cannot place a card relative to itself; " +
			"use --before/--after with a different card, or --position for an absolute index")
	}
	if isBefore {
		beforeID = card.ID
	} else {
		afterID = card.ID
	}
	return position, beforeID, afterID, nil
}

func runAdd(title, description, board, column string, parentCard string, placement cardPlacement, fields []string, strict, nonInteractive, jsonOutput bool) {
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

	// Resolve placement anchors to canonical IDs. Column inference from an anchor
	// happens in the service layer.
	position, beforeID, afterID, err := resolvePlacement(app, boardName, "", placement)
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
			printMissingWantedWarnings(missingWanted)
			Fatal(fmt.Errorf("card not created (strict mode): missing wanted fields: %s", formatMissingWantedFields(missingWanted)))
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
		Position:     position,
		BeforeCard:   beforeID,
		AfterCard:    afterID,
	}

	card, hookResults, err := app.CardService.Add(input)
	if err != nil {
		Fatal(err)
	}

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
		// Check if any option has a description
		hasOptionDescriptions := false
		for _, opt := range mf.Options {
			if opt.Description != "" {
				hasOptionDescriptions = true
				break
			}
		}

		if len(mf.Options) > 0 && hasOptionDescriptions {
			// Expanded multi-line format when any option has a description
			if mf.Description != "" {
				fmt.Fprintf(os.Stderr, "  - %s (%s): %s\n", mf.FieldName, mf.FieldType, mf.Description)
			} else {
				fmt.Fprintf(os.Stderr, "  - %s (%s)\n", mf.FieldName, mf.FieldType)
			}
			fmt.Fprintf(os.Stderr, "    valid values:\n")
			for _, opt := range mf.Options {
				if opt.Description != "" {
					fmt.Fprintf(os.Stderr, "      - %s: %s\n", opt.Value, opt.Description)
				} else {
					fmt.Fprintf(os.Stderr, "      - %s\n", opt.Value)
				}
			}
		} else if len(mf.Options) > 0 {
			// Compact single-line format (no option descriptions)
			values := make([]string, len(mf.Options))
			for i, opt := range mf.Options {
				values[i] = opt.Value
			}
			if mf.Description != "" {
				fmt.Fprintf(os.Stderr, "  - %s (%s): %s\n", mf.FieldName, mf.FieldType, mf.Description)
				fmt.Fprintf(os.Stderr, "    valid values: %s\n", strings.Join(values, ", "))
			} else {
				fmt.Fprintf(os.Stderr, "  - %s (%s): valid values are %s\n",
					mf.FieldName, mf.FieldType, strings.Join(values, ", "))
			}
		} else {
			// No options (string, date, free-set, boolean)
			if mf.Description != "" {
				fmt.Fprintf(os.Stderr, "  - %s (%s): %s\n", mf.FieldName, mf.FieldType, mf.Description)
			} else {
				fmt.Fprintf(os.Stderr, "  - %s (%s)\n", mf.FieldName, mf.FieldType)
			}
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
