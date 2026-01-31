package cli

import (
	"fmt"
	"strings"

	"github.com/amterp/kan/internal/editor"
	"github.com/amterp/ra"
)

func registerComment(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("comment")
	cmd.SetDescription("Manage card comments")

	// comment add
	addCmd := ra.NewCmd("add")
	addCmd.SetDescription("Add a comment to a card")

	ctx.CommentAddCard, _ = ra.NewString("card").
		SetUsage("Card ID or alias").
		Register(addCmd)

	ctx.CommentAddBody, _ = ra.NewString("body").
		SetOptional(true).
		SetUsage("Comment body (opens editor if not provided)").
		Register(addCmd)

	ctx.CommentAddBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		Register(addCmd)

	ctx.CommentAddUsed, _ = cmd.RegisterCmd(addCmd)

	// comment edit
	editCmd := ra.NewCmd("edit")
	editCmd.SetDescription("Edit an existing comment")

	ctx.CommentEditID, _ = ra.NewString("comment-id").
		SetUsage("Comment ID to edit").
		Register(editCmd)

	ctx.CommentEditBody, _ = ra.NewString("body").
		SetOptional(true).
		SetUsage("New comment body (opens editor if not provided)").
		Register(editCmd)

	ctx.CommentEditBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		Register(editCmd)

	ctx.CommentEditUsed, _ = cmd.RegisterCmd(editCmd)

	// comment delete
	deleteCmd := ra.NewCmd("delete")
	deleteCmd.SetDescription("Delete a comment")

	ctx.CommentDeleteID, _ = ra.NewString("comment-id").
		SetUsage("Comment ID to delete").
		Register(deleteCmd)

	ctx.CommentDeleteBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name").
		Register(deleteCmd)

	ctx.CommentDeleteUsed, _ = cmd.RegisterCmd(deleteCmd)

	ctx.CommentUsed, _ = parent.RegisterCmd(cmd)
}

func runCommentAdd(card, body, board string, nonInteractive, jsonOutput bool) {
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

	// Get author
	author, err := app.GetAuthor()
	if err != nil {
		Fatal(err)
	}

	// If body not provided, open editor
	commentBody := body
	if commentBody == "" {
		if nonInteractive {
			Fatal(fmt.Errorf("body is required in non-interactive mode"))
		}

		globalCfg, _ := app.GlobalStore.Load()
		ed := editor.NewEditor(globalCfg)

		edited, err := ed.Edit("")
		if err != nil {
			Fatal(err)
		}

		commentBody = strings.TrimSpace(edited)
		if commentBody == "" {
			fmt.Println("Cancelled (empty comment)")
			return
		}
	}

	// Add comment
	comment, err := app.CardService.AddComment(boardName, card, commentBody, author)
	if err != nil {
		Fatal(err)
	}

	if jsonOutput {
		if err := printJson(CommentOutput{Comment: comment}); err != nil {
			Fatal(err)
		}
		return
	}

	fmt.Printf("Added comment %s\n", comment.ID)
}

func runCommentEdit(commentID, body, board string, nonInteractive bool) {
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

	// Find the card containing this comment to get the existing body
	card, err := app.CardService.FindCommentCard(boardName, commentID)
	if err != nil {
		Fatal(err)
	}

	// Find the comment to get its current body
	var existingBody string
	for _, c := range card.Comments {
		if c.ID == commentID {
			existingBody = c.Body
			break
		}
	}

	// If body not provided, open editor with existing content
	newBody := body
	if newBody == "" {
		if nonInteractive {
			Fatal(fmt.Errorf("body is required in non-interactive mode"))
		}

		globalCfg, _ := app.GlobalStore.Load()
		ed := editor.NewEditor(globalCfg)

		edited, err := ed.Edit(existingBody)
		if err != nil {
			Fatal(err)
		}

		newBody = strings.TrimSpace(edited)
		if newBody == "" {
			fmt.Println("Cancelled (empty comment)")
			return
		}

		if newBody == existingBody {
			fmt.Println("No changes made")
			return
		}
	}

	// Edit comment
	comment, err := app.CardService.EditComment(boardName, commentID, newBody)
	if err != nil {
		Fatal(err)
	}

	fmt.Printf("Updated comment %s\n", comment.ID)
}

func runCommentDelete(commentID, board string, nonInteractive bool) {
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

	// Delete comment
	if err := app.CardService.DeleteComment(boardName, commentID); err != nil {
		Fatal(err)
	}

	fmt.Printf("Deleted comment %s\n", commentID)
}
