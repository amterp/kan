package cli

import (
	"fmt"

	"github.com/amterp/ra"
)

func registerInit(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("init")
	cmd.SetDescription("Initialize Kan in the current repository")

	ctx.InitLocation, _ = ra.NewString("location").
		SetShort("l").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Custom location for .kan directory (relative path)").
		Register(cmd)

	ctx.InitUsed, _ = parent.RegisterCmd(cmd)
}

func runInit(location string) {
	app, err := NewApp(true)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireRepo(); err != nil {
		Fatal(err)
	}

	if err := app.InitService.Initialize(location); err != nil {
		Fatal(err)
	}

	fmt.Println("Initialized Kan repository")
}
