package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/discovery"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/prompt"
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

	ctx.MigrateAll, _ = ra.NewBool("all").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Migrate all projects registered in global config").
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

// projectEntry is a resolved project for --all iteration.
type projectEntry struct {
	name         string
	path         string
	dataLocation string
}

// migrateOutcome describes the result of migrating a single project.
type migrateOutcome int

const (
	outcomeMigrated migrateOutcome = iota
	outcomeUpToDate
	outcomeSkipped
	outcomeFailed
)

func runMigrateAll(dryRun bool, nonInteractive bool) {
	// Load global config via raw TOML to bypass version validation
	// (the config might need migration itself).
	globalConfigPath := config.GlobalConfigPath()
	if globalConfigPath == "" {
		Fatal(fmt.Errorf("no global config found"))
	}

	data, err := os.ReadFile(globalConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			Fatal(fmt.Errorf("no global config found"))
		}
		Fatal(fmt.Errorf("failed to read global config: %w", err))
	}

	var cfg model.GlobalConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		Fatal(fmt.Errorf("failed to parse global config: %w", err))
	}

	if len(cfg.Repos) == 0 {
		fmt.Println("No projects found in global config.")
		return
	}

	// Build sorted project list from Repos, using Projects
	// for display names.
	projects := buildProjectList(&cfg)

	var prompter prompt.Prompter
	if nonInteractive {
		prompter = &prompt.NoopPrompter{}
	} else {
		prompter = prompt.NewHuhPrompter()
	}

	// Migrate global config once upfront.
	migrateGlobalOnce(dryRun)

	// Migrate each project's boards.
	var migrated, upToDate, skipped, failed int
	for _, proj := range projects {
		switch migrateProject(proj, dryRun, nonInteractive, prompter) {
		case outcomeMigrated:
			migrated++
		case outcomeUpToDate:
			upToDate++
		case outcomeSkipped:
			skipped++
		case outcomeFailed:
			failed++
		}
	}

	// Print summary.
	fmt.Println()
	if dryRun {
		summary := fmt.Sprintf("%d need migration, %d up to date",
			migrated, upToDate)
		if skipped > 0 {
			summary += fmt.Sprintf(", %d skipped", skipped)
		}
		if failed > 0 {
			summary += fmt.Sprintf(", %d failed to plan", failed)
		}
		PrintSuccess("Dry run complete: %s.", summary)
	} else {
		summary := fmt.Sprintf("%d migrated, %d up to date",
			migrated, upToDate)
		if skipped > 0 {
			summary += fmt.Sprintf(", %d skipped", skipped)
		}
		if failed > 0 {
			summary += fmt.Sprintf(", %d failed", failed)
		}
		if failed > 0 {
			PrintError("Migration finished with errors: %s.", summary)
			os.Exit(1)
		}
		PrintSuccess("Migration complete: %s.", summary)
	}
}

// buildProjectList resolves the sorted list of projects from
// global config's Repos and Projects maps.
func buildProjectList(cfg *model.GlobalConfig) []projectEntry {
	// Invert Projects map: path -> name.
	// Multiple names can map to the same path, so keep the
	// lexicographically first for deterministic output.
	pathToName := make(map[string]string, len(cfg.Projects))
	for name, path := range cfg.Projects {
		if existing, ok := pathToName[path]; !ok || name < existing {
			pathToName[path] = name
		}
	}

	var projects []projectEntry
	for path, repoCfg := range cfg.Repos {
		name := pathToName[path]
		if name == "" {
			name = path // fallback to path if no display name
		}
		projects = append(projects, projectEntry{
			name:         name,
			path:         path,
			dataLocation: repoCfg.DataLocation,
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].name < projects[j].name
	})

	return projects
}

// migrateGlobalOnce plans and executes global config migration.
func migrateGlobalOnce(dryRun bool) {
	// We need a MigrateService just for global planning - paths
	// don't matter since PlanGlobalMigration uses GlobalConfigPath.
	svc := service.NewMigrateService(config.NewPaths("", ""))
	globalPlan, err := svc.PlanGlobalMigration()
	if err != nil {
		PrintWarning("Failed to plan global config migration: %v", err)
		return
	}

	if globalPlan == nil || !globalPlan.NeedsMigration {
		fmt.Println(RenderMuted("Global config: up to date"))
		return
	}

	plan := &service.MigrationPlan{GlobalConfig: globalPlan}
	if dryRun {
		fmt.Println(RenderBold("Global config (dry run):"))
		_ = svc.Execute(plan, true)
	} else {
		if err := svc.Execute(plan, false); err != nil {
			PrintWarning("Failed to migrate global config: %v", err)
		}
	}
}

// migrateProject plans and optionally executes migration for a
// single project's boards. Returns the outcome for summary tracking.
func migrateProject(proj projectEntry, dryRun bool, nonInteractive bool, prompter prompt.Prompter) migrateOutcome {
	header := fmt.Sprintf("Project: %s (%s)", RenderBold(proj.name), proj.path)

	// Check if project path exists on disk.
	if _, err := os.Stat(proj.path); os.IsNotExist(err) {
		PrintWarning("Skipping %q (%s) - path not found", proj.name, proj.path)
		return outcomeSkipped
	}

	paths := config.NewPaths(proj.path, proj.dataLocation)
	svc := service.NewMigrateService(paths)

	plan, err := svc.PlanBoardsOnly()
	if err != nil {
		PrintWarning("Skipping %q - failed to plan: %v", proj.name, err)
		return outcomeFailed
	}

	if !plan.HasChanges() {
		fmt.Printf("\n%s: %s\n", header, RenderMuted("up to date"))
		return outcomeUpToDate
	}

	// Print what would change.
	fmt.Printf("\n%s\n", header)
	printBoardSummary(plan)

	if dryRun {
		// In dry-run, count projects that *would* be migrated.
		return outcomeMigrated
	}

	// In interactive mode, confirm before migrating.
	if !nonInteractive {
		confirmed, err := prompter.Confirm(
			fmt.Sprintf("Migrate %q?", proj.name), true)
		if err != nil {
			PrintWarning("Skipping %q - prompt failed: %v", proj.name, err)
			return outcomeFailed
		}
		if !confirmed {
			fmt.Printf("  %s\n", RenderMuted("Skipped"))
			return outcomeSkipped
		}
	}

	if err := svc.Execute(plan, false); err != nil {
		PrintWarning("Failed to migrate %q: %v", proj.name, err)
		return outcomeFailed
	}
	return outcomeMigrated
}

// printBoardSummary prints a concise summary of board migrations
// in a plan.
func printBoardSummary(plan *service.MigrationPlan) {
	for _, board := range plan.Boards {
		cardsToMigrate := 0
		for _, card := range board.Cards {
			if card.FromVersion != card.ToVersion || card.RemoveColumn {
				cardsToMigrate++
			}
		}

		if !board.NeedsMigration && cardsToMigrate == 0 {
			continue
		}

		parts := []string{}
		if board.NeedsMigration {
			fromDisplay := board.FromSchema
			if fromDisplay == "" {
				fromDisplay = "(missing)"
			}
			parts = append(parts,
				fmt.Sprintf("config %s -> %s",
					fromDisplay, board.ToSchema))
		}
		if cardsToMigrate > 0 {
			parts = append(parts,
				fmt.Sprintf("%d cards", cardsToMigrate))
		}

		detail := ""
		for i, p := range parts {
			if i > 0 {
				detail += ", "
			}
			detail += p
		}
		fmt.Printf("  Board %q: %s\n", board.BoardName, detail)
	}
}
