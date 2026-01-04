package cli

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/editor"
	"github.com/amterp/kan/internal/model"
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

	ctx.EditUsed, _ = parent.RegisterCmd(cmd)
}

func runEdit(idOrAlias, board string) {
	app, err := NewApp(true)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	// Resolve board
	boardName, err := app.BoardResolver.Resolve(board, true)
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

	// Select field to edit
	fields := []string{"title", "description", "column", "labels"}
	field, err := app.Prompter.Select("Select field to edit", fields)
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
	case "labels":
		editLabels(app, boardName, card, boardCfg)
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

	card.Column = newColumn
	if err := app.CardService.Update(boardName, card); err != nil {
		Fatal(err)
	}

	fmt.Printf("Moved card to %q\n", newColumn)
}

func editLabels(app *App, boardName string, card *model.Card, boardCfg *model.BoardConfig) {
	if len(boardCfg.Labels) == 0 {
		Fatal(fmt.Errorf("no labels defined in board"))
	}

	labels := make([]string, len(boardCfg.Labels))
	for i, lbl := range boardCfg.Labels {
		labels[i] = lbl.Name
	}

	newLabels, err := app.Prompter.MultiSelect("Select labels", labels)
	if err != nil {
		Fatal(err)
	}

	card.Labels = newLabels
	if err := app.CardService.Update(boardName, card); err != nil {
		Fatal(err)
	}

	if len(newLabels) == 0 {
		fmt.Println("Cleared labels")
	} else {
		fmt.Printf("Updated labels: %s\n", strings.Join(newLabels, ", "))
	}
}
