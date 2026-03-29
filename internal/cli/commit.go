package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/amterp/ra"
)

func registerCommit(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("commit")
	cmd.SetDescription("Stage and commit kan data files to git (leaves other staged changes untouched)")

	ctx.CommitMessage, _ = ra.NewString("message").
		SetShort("m").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage(`Commit message (default: "chore: update kan files")`).
		Register(cmd)

	ctx.CommitUsed, _ = parent.RegisterCmd(cmd)
}

// conflictCodes are the two-character porcelain status codes that indicate
// an unresolved merge/rebase conflict. Staging files in these states would
// commit conflict markers rather than valid content.
var conflictCodes = map[string]bool{
	"UU": true, "AA": true, "DD": true,
	"AU": true, "UA": true, "DU": true, "UD": true,
}

func runCommit(message string) {
	app, err := NewApp(false)
	if err != nil {
		Fatal(err)
	}
	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}
	if !app.GitClient.IsRepoAt(app.ProjectRoot) {
		Fatal(fmt.Errorf("not a git repository"))
	}

	kanRelPath, err := filepath.Rel(app.ProjectRoot, app.Paths.KanRoot())
	if err != nil {
		Fatal(fmt.Errorf("failed to resolve kan path: %w", err))
	}

	status, err := app.GitClient.StatusPorcelain(app.ProjectRoot, kanRelPath)
	if err != nil {
		Fatal(fmt.Errorf("failed to check git status: %w", err))
	}
	if status == "" {
		PrintSuccess("No kan changes to commit")
		return
	}

	// Abort if any kan files have unresolved conflicts - staging them would
	// commit conflict markers rather than valid JSON.
	for _, line := range strings.Split(status, "\n") {
		if len(line) >= 2 && conflictCodes[line[:2]] {
			Fatal(fmt.Errorf("unresolved conflicts in kan files - resolve them before committing"))
		}
	}

	if message == "" {
		message = "chore: update kan files"
	}

	if err := app.GitClient.Add(app.ProjectRoot, kanRelPath); err != nil {
		Fatal(err)
	}
	if err := app.GitClient.Commit(app.ProjectRoot, message, kanRelPath); err != nil {
		Fatal(err)
	}

	PrintSuccess("Committed kan files: %q", message)
}
