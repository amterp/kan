package cli

import (
	"fmt"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/discovery"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/ra"
)

func registerMigrate(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("migrate")
	cmd.SetDescription("Migrate board data to current schema version")

	ctx.MigrateDryRun, _ = ra.NewBool("dry-run").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Show what would be changed without modifying files").
		Register(cmd)

	ctx.MigrateUsed, _ = parent.RegisterCmd(cmd)
}

func runMigrate(dryRun bool) {
	// Discover project without version validation
	// Pass nil for global config to avoid loading it (which might fail version checks)
	result, err := discovery.DiscoverProject(&model.GlobalConfig{})
	if err != nil {
		Fatal(err)
	}
	if result == nil {
		Fatal(fmt.Errorf("no .kan directory found (run 'kan init' first)"))
	}

	paths := config.NewPaths(result.ProjectRoot, result.DataLocation)
	migrateService := service.NewMigrateService(paths)

	plan, err := migrateService.Plan()
	if err != nil {
		Fatal(err)
	}

	if !plan.HasChanges() {
		PrintSuccess("Everything is up to date. No migration needed.")
		return
	}

	if dryRun {
		fmt.Println(RenderBold("Migration plan (dry run):"))
		fmt.Println()
	}

	if err := migrateService.Execute(plan, dryRun); err != nil {
		Fatal(err)
	}

	if !dryRun {
		fmt.Println()
		PrintSuccess("Migration complete.")
		fmt.Println(RenderMuted("Tip: Commit this migration separately. Use 'git blame --ignore-rev' to hide bulk changes."))
	}
}
