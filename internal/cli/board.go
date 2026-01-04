package cli

import (
	"fmt"

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

	fmt.Printf("Created board %q\n", name)
}

func runBoardList() {
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

	if len(boards) == 0 {
		fmt.Println("No boards found")
		return
	}

	for _, board := range boards {
		fmt.Println(board)
	}
}
