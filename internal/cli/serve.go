package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/amterp/kan/internal/api"
	"github.com/amterp/ra"
)

func registerServe(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("serve")
	cmd.SetDescription("Start web interface")

	ctx.ServePort, _ = ra.NewInt("port").
		SetOptional(true).
		SetDefault(8080).
		SetShort("p").
		SetFlagOnly(true).
		SetUsage("Port to listen on").
		Register(cmd)

	ctx.ServeNoOpen, _ = ra.NewBool("no-open").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Don't open browser automatically").
		Register(cmd)

	ctx.ServeUsed, _ = parent.RegisterCmd(cmd)
}

func runServe(port int, noOpen bool) {
	app, err := NewApp(false)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	handler := api.NewHandler(
		app.CardService,
		app.BoardService,
		app.CardStore,
		app.BoardStore,
		app.GetCreator(),
	)

	server := api.NewServer(handler, port)

	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("Kan web server running at %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

	if !noOpen {
		openBrowser(url)
	}

	if err := server.Start(); err != nil {
		Fatal(err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}
