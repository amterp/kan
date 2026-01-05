package cli

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/editor"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/ra"
)

func registerEdit(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("edit")
	cmd.SetDescription("Edit an existing card")

	ctx.EditCard, _ = ra.NewString("card").
		SetUsage("Card ID or alias").
		Register(cmd)

	ctx.EditBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		Register(cmd)

	ctx.EditTitle, _ = ra.NewString("title").
		SetShort("t").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Set card title").
		Register(cmd)

	ctx.EditDescription, _ = ra.NewString("description").
		SetShort("d").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Set card description").
		Register(cmd)

	ctx.EditColumn, _ = ra.NewString("column").
		SetShort("c").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Move card to column").
		Register(cmd)

	ctx.EditParent, _ = ra.NewString("parent").
		SetShort("p").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Set parent card ID or alias").
		Register(cmd)

	ctx.EditAlias, _ = ra.NewString("alias").
		SetShort("a").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Set explicit alias").
		Register(cmd)

	ctx.EditFields, _ = ra.NewStringSlice("field").
		SetShort("f").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Set custom field (key=value format, repeatable)").
		Register(cmd)

	ctx.EditUsed, _ = parent.RegisterCmd(cmd)
}

func runEdit(idOrAlias, board string, title, description, column string,
	parent, alias string, fields []string, nonInteractive bool) {

	// Check if any flags were provided
	hasFlags := title != "" || description != "" || column != "" ||
		parent != "" || alias != "" || len(fields) > 0

	if !hasFlags && nonInteractive {
		Fatal(fmt.Errorf("no fields specified to edit (use -t, -d, -c, -p, -a, or -f flags)"))
	}

	app, err := NewApp(!nonInteractive)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	// Resolve board (allow interactive only if not in non-interactive mode)
	boardName, err := app.BoardResolver.Resolve(board, !nonInteractive)
	if err != nil {
		Fatal(err)
	}

	// Resolve card
	card, err := app.CardResolver.Resolve(boardName, idOrAlias)
	if err != nil {
		Fatal(err)
	}

	// Get board config for column/label options
	boardCfg, err := app.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	if hasFlags {
		// Non-interactive path: apply flags directly
		runEditNonInteractive(app, boardName, card, title, description, column,
			parent, alias, fields)
	} else {
		// Interactive path: existing menu-based editing
		runEditInteractive(app, boardName, card, boardCfg)
	}
}

// runEditNonInteractive applies CLI flag changes to the card.
func runEditNonInteractive(app *App, boardName string, card *model.Card,
	title, description, column string,
	parent, alias string, fields []string) {

	input := service.EditCardInput{
		BoardName:     boardName,
		CardIDOrAlias: card.ID,
	}

	if title != "" {
		input.Title = &title
	}
	if description != "" {
		input.Description = &description
	}
	if column != "" {
		input.Column = &column
	}
	if parent != "" {
		input.Parent = &parent
	}
	if alias != "" {
		input.Alias = &alias
	}
	if len(fields) > 0 {
		parsed, err := parseCustomFields(fields)
		if err != nil {
			Fatal(err)
		}
		input.CustomFields = parsed
	}

	updatedCard, err := app.CardService.Edit(input)
	if err != nil {
		Fatal(err)
	}

	fmt.Printf("Updated card %s\n", updatedCard.ID)
}

// parseCustomFields converts ["key=value", ...] to map[string]string.
func parseCustomFields(fields []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, f := range fields {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid field format %q (expected key=value)", f)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("empty field name in %q", f)
		}
		result[key] = value
	}
	return result, nil
}

// runEditInteractive runs the interactive menu-based editing flow.
func runEditInteractive(app *App, boardName string, card *model.Card, boardCfg *model.BoardConfig) {
	// Select field to edit
	fieldOptions := []string{"title", "description", "column"}
	field, err := app.Prompter.Select("Select field to edit", fieldOptions)
	if err != nil {
		Fatal(err)
	}

	// Edit the field
	switch field {
	case "title":
		editTitle(app, boardName, card)
	case "description":
		editDescription(app, boardName, card)
	case "column":
		editColumn(app, boardName, card, boardCfg)
	}
}

func editTitle(app *App, boardName string, card *model.Card) {
	globalCfg, _ := app.GlobalStore.Load()
	ed := editor.NewEditor(globalCfg)

	newTitle, err := ed.Edit(card.Title)
	if err != nil {
		Fatal(err)
	}

	newTitle = strings.TrimSpace(newTitle)
	if newTitle == "" {
		Fatal(fmt.Errorf("title cannot be empty"))
	}

	if newTitle == card.Title {
		fmt.Println("No changes made")
		return
	}

	if err := app.CardService.UpdateTitle(boardName, card, newTitle); err != nil {
		Fatal(err)
	}

	fmt.Printf("Updated title to %q\n", newTitle)
}

func editDescription(app *App, boardName string, card *model.Card) {
	globalCfg, _ := app.GlobalStore.Load()
	ed := editor.NewEditor(globalCfg)

	newDesc, err := ed.Edit(card.Description)
	if err != nil {
		Fatal(err)
	}

	newDesc = strings.TrimSpace(newDesc)

	if newDesc == card.Description {
		fmt.Println("No changes made")
		return
	}

	card.Description = newDesc
	if err := app.CardService.Update(boardName, card); err != nil {
		Fatal(err)
	}

	fmt.Println("Updated description")
}

func editColumn(app *App, boardName string, card *model.Card, boardCfg *model.BoardConfig) {
	columns := make([]string, len(boardCfg.Columns))
	for i, col := range boardCfg.Columns {
		columns[i] = col.Name
	}

	newColumn, err := app.Prompter.Select("Select column", columns)
	if err != nil {
		Fatal(err)
	}

	if newColumn == card.Column {
		fmt.Println("No changes made")
		return
	}

	if err := app.CardService.MoveCard(boardName, card.ID, newColumn); err != nil {
		Fatal(err)
	}

	fmt.Printf("Moved card to %q\n", newColumn)
}
