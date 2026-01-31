package cli

import (
	"fmt"
	"net"
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
		SetDefault(5260).
		SetShort("p").
		SetFlagOnly(true).
		SetUsage("Port to listen on (will try incrementally if in use)").
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

	creatorName, err := app.GetAuthor()
	if err != nil {
		Fatal(err)
	}

	ctx := &api.ProjectContext{
		Paths:        app.Paths,
		BoardStore:   app.BoardStore,
		CardStore:    app.CardStore,
		ProjectStore: app.ProjectStore,
		CardService:  app.CardService,
		BoardService: app.BoardService,
		Creator:      creatorName,
		ProjectRoot:  app.ProjectRoot,
	}

	handler := api.NewHandler(app.GlobalStore, ctx)

	// Find an available port starting from the requested one
	actualPort := findAvailablePort(port)

	server := api.NewServer(handler, actualPort, app.ProjectRoot)

	url := fmt.Sprintf("http://localhost:%d", actualPort)

	// Display styled server info
	content := fmt.Sprintf("Kan Web Server\n\n%s %s\n\nPress Ctrl+C to stop",
		RenderMuted("→"), RenderURL(url))
	fmt.Println(Box(content))

	if !noOpen {
		openBrowser(url)
	}

	if err := server.Start(); err != nil {
		Fatal(err)
	}
}

// findAvailablePort tries ports starting from startPort until it finds one that's available.
func findAvailablePort(startPort int) int {
	maxAttempts := 100
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		if isPortAvailable(port) {
			return port
		}
	}
	// If we couldn't find a port after maxAttempts, return the original and let it fail naturally
	return startPort
}

// isPortAvailable checks if a port is available by attempting to listen on it.
func isPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
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
