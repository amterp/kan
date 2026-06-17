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
		SetUsage("Port to listen on (auto-increments if unspecified and in use; errors out if explicitly set and unavailable)").
		Register(cmd)

	ctx.ServeHost, _ = ra.NewString("host").
		SetOptional(true).
		SetDefault("127.0.0.1").
		SetFlagOnly(true).
		SetUsage("Interface to bind to. Defaults to 127.0.0.1 (local only). Use 0.0.0.0 to allow other devices on your network to connect - only do this on trusted networks.").
		Register(cmd)

	ctx.ServeNoOpen, _ = ra.NewBool("no-open").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Don't open browser automatically").
		Register(cmd)

	ctx.ServeUsed, _ = parent.RegisterCmd(cmd)
}

func runServe(host string, port int, portExplicit bool, noOpen bool) {
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

	// If user explicitly set --port, honor it exactly; otherwise auto-increment.
	var actualPort int
	if portExplicit {
		if !isPortAvailable(host, port) {
			Fatal(fmt.Errorf("port %d is already in use", port))
		}
		actualPort = port
	} else {
		actualPort = findAvailablePort(host, port)
	}

	server := api.NewServer(handler, host, actualPort, app.Paths.KanRoot())

	// "0.0.0.0" / "::" aren't connectable hostnames on most systems; show
	// "localhost" instead since it resolves to the loopback interface that's
	// always included in those binds.
	displayHost := host
	if host == "0.0.0.0" || host == "::" {
		displayHost = "localhost"
	}
	url := fmt.Sprintf("http://%s:%d", displayHost, actualPort)

	// Display styled server info
	content := fmt.Sprintf("Kan Web Server\n\n%s %s\n\nPress Ctrl+C to stop",
		RenderMuted("→"), RenderURL(url))
	if host == "0.0.0.0" || host == "::" {
		content = fmt.Sprintf("Kan Web Server\n\n%s %s\n\n%s This is reachable from other devices on your network.\n\nPress Ctrl+C to stop",
			RenderMuted("→"), RenderURL(url), RenderMuted("Warning:"))
	}
	fmt.Println(Box(content))

	if !noOpen {
		openBrowser(url)
	}

	if err := server.Start(); err != nil {
		Fatal(err)
	}
}

// findAvailablePort tries ports starting from startPort until it finds one that's available
// on the given host.
func findAvailablePort(host string, startPort int) int {
	maxAttempts := 100
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		if isPortAvailable(host, port) {
			return port
		}
	}
	// If we couldn't find a port after maxAttempts, return the original and let it fail naturally
	return startPort
}

// isPortAvailable checks if a port is available on the given host by attempting to listen on it.
func isPortAvailable(host string, port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
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
