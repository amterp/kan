package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/amterp/ra"
)

// columnNameRegex validates column names: lowercase alphanumeric and hyphens.
var columnNameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func registerInit(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("init")
	cmd.SetDescription("Initialize Kan in the current directory")

	ctx.InitLocation, _ = ra.NewString("location").
		SetShort("l").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Custom location for .kan directory (relative path)").
		Register(cmd)

	ctx.InitColumns, _ = ra.NewString("columns").
		SetShort("c").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Comma-separated list of column names (e.g., backlog,todo,doing,done)").
		Register(cmd)

	ctx.InitName, _ = ra.NewString("name").
		SetShort("n").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name (default: main)").
		Register(cmd)

	ctx.InitUsed, _ = parent.RegisterCmd(cmd)
}

// parseColumns parses and validates a comma-separated list of column names.
// Returns nil if the input is empty (use defaults).
func parseColumns(columnsStr string) ([]string, error) {
	if columnsStr == "" {
		return nil, nil
	}

	var columns []string
	seen := make(map[string]bool)

	for _, col := range strings.Split(columnsStr, ",") {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		if !columnNameRegex.MatchString(col) {
			return nil, fmt.Errorf("invalid column name %q (must be lowercase alphanumeric with hyphens, e.g., 'in-progress')", col)
		}
		if seen[col] {
			return nil, fmt.Errorf("duplicate column name %q", col)
		}
		seen[col] = true
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("at least one column name required when using --columns")
	}
	return columns, nil
}

func runInit(location, boardName, columnsStr string) {
	columns, err := parseColumns(columnsStr)
	if err != nil {
		Fatal(err)
	}

	app, err := NewApp(true)
	if err != nil {
		Fatal(err)
	}

	if err := app.InitService.Initialize(location, boardName, columns); err != nil {
		Fatal(err)
	}

	fmt.Println("Initialized Kan in current directory")
}
