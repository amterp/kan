package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/creator"
	"github.com/amterp/kan/internal/discovery"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/git"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/prompt"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/resolver"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
	"github.com/amterp/kan/internal/version"
)

// App holds all the dependencies for the CLI.
// Uses interfaces for testability.
type App struct {
	GitClient     *git.Client
	GlobalStore   store.GlobalStore
	ProjectStore  store.ProjectStore
	Paths         *config.Paths
	BoardStore    store.BoardStore
	CardStore     store.CardStore
	Prompter      prompt.Prompter
	InitService   *service.InitService
	BoardService  *service.BoardService
	CardService   *service.CardService
	LinkService   *service.LinkService
	AliasService  *service.AliasService
	HookService   *service.HookService
	BoardResolver *resolver.BoardResolver
	CardResolver  *resolver.CardResolver
	ProjectRoot   string
	// OtherProject is true for a registered project opened for reading
	// from elsewhere (openProjectForReading), never written.
	OtherProject bool
	// UsingGlobalBoard is true when the App was built via -g (targeting the
	// designated global board). Handlers use it to surface the target in output.
	UsingGlobalBoard bool

	globalCfg *model.GlobalConfig
}

// AppOptions configures how NewAppWithOptions wires up the App.
type AppOptions struct {
	// Interactive selects the HuhPrompter (vs the NoopPrompter that fails on prompts).
	Interactive bool
	// UseGlobalBoard bypasses cwd discovery and targets the designated global
	// board's project instead (see `kan global set`). Errors if none is set.
	UseGlobalBoard bool
	// ReadOnly marks a command that only reads. Kan then writes nothing
	// under the project: older schemas are migrated in memory, a missing
	// project config is not created, and the stores refuse writes. The
	// global config may still be written (registration, its own migration).
	ReadOnly bool
}

// NewApp creates a new App with all dependencies wired up.
// If interactive is false, uses NoopPrompter that fails on prompts.
func NewApp(interactive bool) (*App, error) {
	return NewAppWithOptions(AppOptions{Interactive: interactive})
}

// NewAppWithOptions creates a new App, optionally retargeted at the designated
// global board instead of the project discovered from the cwd.
func NewAppWithOptions(opts AppOptions) (*App, error) {
	gitClient := git.NewClient()
	globalStore := store.NewGlobalStore()

	// Auto-migrate global config before loading (schema validation would
	// otherwise reject old versions).
	if err := autoMigrateGlobal(); err != nil {
		var schemaErr *version.SchemaVersionError
		if errors.As(err, &schemaErr) {
			// Future version - hard fail with "upgrade Kan" message
			return nil, err
		}
		PrintWarning("failed to auto-migrate global config: %v", err)
	}

	// Load global config with warnings (don't silently ignore errors)
	globalCfg, err := globalStore.Load()
	if err != nil {
		PrintWarning("failed to load global config: %v", err)
		globalCfg = nil
	}

	var projectRoot, dataLocation, globalBoardName string

	if opts.UseGlobalBoard {
		// Global mode: resolve the designated board's project instead of the cwd.
		// We deliberately skip worktree resolution and auto-registration - the
		// recorded path is already a canonical project root.
		if globalCfg == nil || globalCfg.GlobalBoard == nil {
			return nil, &kanerr.NoGlobalBoardError{}
		}
		ref := globalCfg.GlobalBoard
		projectRoot = ref.Path
		globalBoardName = ref.Board
		if repoCfg := globalCfg.GetRepoConfig(ref.Path); repoCfg != nil {
			dataLocation = repoCfg.DataLocation
		}
	} else {
		result, derr := discoverProject(globalCfg, gitClient)
		if derr != nil {
			return nil, derr
		}

		if result != nil {
			projectRoot = result.ProjectRoot
			dataLocation = result.DataLocation

			// Clean up stale global config entry for the worktree path
			if result.ResolvedFromWorktree && globalCfg != nil && result.OriginalWorktreeRoot != "" {
				cleanupStaleWorktreeEntry(globalStore, globalCfg, result.OriginalWorktreeRoot)
			}

			// Auto-register unregistered projects (but not worktree paths)
			if !result.WasRegistered && !result.ResolvedFromWorktree && globalCfg != nil {
				registerProject(globalStore, globalCfg, projectRoot, dataLocation)
			}
		}
		// projectRoot may be empty - that's OK, RequireKan() will catch it
	}

	paths := config.NewPaths(projectRoot, dataLocation)
	var prompter prompt.Prompter = &prompt.NoopPrompter{}
	if opts.Interactive {
		prompter = prompt.NewHuhPrompter()
	}
	app, err := newProjectApp(paths, opts.ReadOnly, prompter, globalStore, globalCfg, gitClient)
	if err != nil {
		return nil, err
	}
	app.UsingGlobalBoard = opts.UseGlobalBoard

	// In global mode, fail early with a clear message if the designation has gone
	// stale (project moved/deleted, or board removed) rather than surfacing a
	// generic "not initialized" later.
	if opts.UseGlobalBoard {
		if !app.BoardStore.Exists(globalBoardName) {
			return nil, &kanerr.StaleGlobalBoardError{Board: globalBoardName, Path: util.PrettyPath(projectRoot)}
		}
		app.BoardResolver.SetPreferredBoard(globalBoardName)
	}

	// Ensure project config exists with ID (graceful upgrade for older projects)
	if projectRoot != "" && !opts.ReadOnly {
		defaultName := filepath.Base(projectRoot)
		if err := app.ProjectStore.EnsureInitialized(defaultName); err != nil {
			// Non-fatal: log warning but continue
			PrintWarning("failed to initialize project config: %v", err)
		}
	}

	// Set up hook service if we have a project root
	if projectRoot != "" {
		app.HookService = service.NewHookService(projectRoot)
		app.CardService.SetHookService(app.HookService)
	}

	elsewhere := app.elsewhere(app)
	app.LinkService.SetElsewhere(elsewhere, app.ProjectStore)
	app.CardService.SetElsewhere(elsewhere, app.ProjectStore)

	return app, nil
}

// elsewhere lets link lookups from project reach boards in other registered
// projects, read-only. project is the App itself, or another project it
// opened; its [linked_boards] describes boards that aren't on this machine.
func (a *App) elsewhere(project *App) *service.Elsewhere {
	e := &service.Elsewhere{
		Locator: a.locator(),
		Open: func(root, dataLocation string) (store.BoardStore, store.CardStore, error) {
			p, err := a.openProjectForReading(root, dataLocation)
			if err != nil {
				return nil, nil, err
			}
			return p.BoardStore, p.CardStore, nil
		},
		Remote: func(root string) string {
			return git.StripCredentials(git.FetchURL(a.GitClient.Remotes(root)))
		},
	}
	if a.globalCfg != nil {
		e.Projects = registry.Visible(a.globalCfg)
	}
	if a.RequireKan() == nil && !slices.ContainsFunc(e.Projects, func(p registry.Entry) bool { return registry.SamePath(p.Path, a.ProjectRoot) }) {
		e.Projects = append(e.Projects, registry.Entry{Path: a.ProjectRoot, DataLocation: a.Paths.DataLocation()})
	}
	if project.ProjectRoot != "" {
		if cfg, err := project.ProjectStore.Load(); err == nil {
			e.LinkedBoards = cfg.LinkedBoards
		}
	}
	return e
}

// newProjectApp wires the stores, services and resolvers for the project at
// paths. It runs no hooks and creates no project config; callers add those.
func newProjectApp(paths *config.Paths, readOnly bool, prompter prompt.Prompter, globalStore store.GlobalStore,
	globalCfg *model.GlobalConfig, gitClient *git.Client) (*App, error) {
	stores, err := openProjectStores(paths, readOnly)
	if err != nil {
		return nil, err
	}
	boardStore, cardStore := stores.board, stores.card

	aliasService := service.NewAliasService(cardStore)
	return &App{
		GitClient:     gitClient,
		GlobalStore:   globalStore,
		ProjectStore:  stores.project,
		Paths:         paths,
		BoardStore:    boardStore,
		CardStore:     cardStore,
		Prompter:      prompter,
		InitService:   service.NewInitService(globalStore),
		BoardService:  service.NewBoardService(boardStore, cardStore),
		CardService:   service.NewCardService(cardStore, boardStore, aliasService),
		LinkService:   service.NewLinkService(cardStore, boardStore),
		AliasService:  aliasService,
		BoardResolver: resolver.NewBoardResolver(boardStore, globalStore, prompter, paths.ProjectRoot()),
		CardResolver:  resolver.NewCardResolver(cardStore),
		ProjectRoot:   paths.ProjectRoot(),
		globalCfg:     globalCfg,
	}, nil
}

// openProjectForReading opens a registered project other than the current
// one. It reads through read-only stores with in-memory migration, and runs
// no hooks, registers nothing and creates no project config. Data newer than
// this kan, or a migration that can't be planned, fails with an error naming
// the project.
func (a *App) openProjectForReading(root, dataLocation string) (*App, error) {
	paths := config.NewPaths(root, dataLocation)
	// The caller's prompter, so pickers in that project prompt exactly when
	// the command could (it is the no-op prompter under -I).
	project, err := newProjectApp(paths, true, a.Prompter, a.GlobalStore, a.globalCfg, a.GitClient)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", root, err)
	}
	project.OtherProject = true
	return project, nil
}

// discoverProject finds the project for the working directory (VCS-agnostic),
// and from a linked git worktree redirects to the main worktree's board.
// Returns nil when there is no project. An error is real, e.g. the global
// config names a path whose data is gone.
func discoverProject(globalCfg *model.GlobalConfig, worktrees discovery.WorktreeResolver) (*discovery.Result, error) {
	result, err := discovery.DiscoverProject(globalCfg)
	if err != nil || result == nil {
		return result, err
	}

	result, err = discovery.ResolveWorktree(result, worktrees, globalCfg, isWorktreeIndependent)
	if err != nil {
		return nil, err
	}
	if result.ResolvedFromWorktree {
		PrintInfo("Using board from main worktree at %s", result.ProjectRoot)
	}
	return result, nil
}

// projectStores are the stores for one project.
type projectStores struct {
	board   *store.FileBoardStore
	card    *store.FileCardStore
	project *store.FileProjectStore
}

// openProjectStores opens a project's stores. For a write, older data is
// migrated on disk first. For a read, it is migrated in memory and the
// stores refuse to write. Either way, data newer than this kan is an error
// that tells the user to upgrade.
func openProjectStores(paths *config.Paths, readOnly bool) (projectStores, error) {
	if !readOnly {
		if paths.ProjectRoot() != "" {
			if err := autoMigrateProject(paths); err != nil {
				return projectStores{}, err
			}
		}
		return projectStores{
			board:   store.NewBoardStore(paths),
			card:    store.NewCardStore(paths),
			project: store.NewProjectStore(paths),
		}, nil
	}

	var overlay *store.Overlay
	if paths.ProjectRoot() != "" {
		var err error
		overlay, err = service.NewQuietMigrateService(paths).LoadProjectOverlay()
		if err != nil {
			return projectStores{}, err
		}
	}
	return projectStores{
		board:   store.NewReadOnlyBoardStore(paths, overlay),
		card:    store.NewReadOnlyCardStore(paths, overlay),
		project: store.NewReadOnlyProjectStore(paths, overlay),
	}, nil
}

// NewAppWithoutDiscovery creates a minimal App without running project discovery.
// Used by init command when discovery fails due to stale global config.
func NewAppWithoutDiscovery() (*App, error) {
	globalStore := store.NewGlobalStore()

	// Just need InitService for the init command
	initService := service.NewInitService(globalStore)

	return &App{
		GlobalStore: globalStore,
		InitService: initService,
	}, nil
}

// registerProject auto-registers a discovered but unregistered project in
// global config, following the registry's rules for temp directories and
// second copies of a board.
func registerProject(globalStore store.GlobalStore, globalCfg *model.GlobalConfig, projectRoot, dataLocation string) {
	result := registry.Register(globalCfg, projectRoot, dataLocation)
	if !result.Registered {
		return
	}
	if msg := result.Message(); msg != "" {
		PrintWarning("%s", msg)
	}

	// Best effort - don't fail if we can't save
	_ = globalStore.Save(globalCfg)
}

// isWorktreeIndependent checks if the project at the given root has opted out
// of worktree sharing via the worktree_independent flag in project config.
func isWorktreeIndependent(projectRoot, dataLocation string) bool {
	paths := config.NewPaths(projectRoot, dataLocation)
	ps := store.NewProjectStore(paths)
	cfg, err := ps.Load()
	if err != nil {
		return false
	}
	return cfg.WorktreeIndependent
}

// cleanupStaleWorktreeEntry removes a worktree path from global config if it
// was auto-registered before worktree support existed.
func cleanupStaleWorktreeEntry(globalStore store.GlobalStore, globalCfg *model.GlobalConfig, worktreePath string) {
	if globalCfg.GetRepoConfig(worktreePath) != nil {
		globalCfg.RemoveRepoConfig(worktreePath)
		_ = globalStore.Save(globalCfg)
	}
}

// autoMigrateGlobal checks the global config for outdated schema versions and
// migrates transparently. Returns a SchemaVersionError for future versions
// (user needs a newer Kan), or a wrapped error if migration fails.
func autoMigrateGlobal() error {
	svc := service.NewQuietMigrateService(config.NewPaths("", ""))
	globalPlan, err := svc.PlanGlobalMigration()
	if err != nil {
		return fmt.Errorf("failed to check global config version: %w", err)
	}

	if globalPlan == nil || !globalPlan.NeedsMigration {
		return nil
	}

	plan := &service.MigrationPlan{GlobalConfig: globalPlan}
	if err := plan.FutureVersionError(); err != nil {
		return err
	}

	if err := svc.Execute(plan, false); err != nil {
		return fmt.Errorf("auto-migration of global config failed: %w", err)
	}

	PrintInfo("Auto-migrated global config to %s", version.CurrentGlobalSchema())
	return nil
}

// autoMigrateProject checks a project's config, boards and cards for
// outdated schema versions and migrates transparently. Returns a
// SchemaVersionError for future versions, or a wrapped error if migration
// fails.
func autoMigrateProject(paths *config.Paths) error {
	migrated, err := service.NewQuietMigrateService(paths).MigrateProject()
	if err != nil {
		var schemaErr *version.SchemaVersionError
		if errors.As(err, &schemaErr) {
			return err
		}
		return fmt.Errorf("auto-migration failed: %w", err)
	}
	if !migrated {
		return nil
	}

	PrintInfo("Auto-migrated project data to current schema.")
	fmt.Fprintln(os.Stderr, RenderMuted("  Tip: commit the migrated files separately."))
	return nil
}

// PrintTarget surfaces a board outside the current project: the -g board,
// so a mistaken designation is immediately visible, or another project's
// board read through a board ID or a board:card reference. Writes to stderr
// (like other status output), so it never pollutes --json.
func (a *App) PrintTarget(boardName string) {
	label := "board"
	switch {
	case a.UsingGlobalBoard:
		label = "global board"
	case !a.OtherProject:
		return
	}
	PrintInfo("%s %q %s", label, boardName, RenderMuted("("+util.PrettyPath(a.ProjectRoot)+")"))
}

// RequireKan ensures Kan is initialized in the current project.
func (a *App) RequireKan() error {
	if a.ProjectRoot == "" {
		return &kanerr.NotInitializedError{}
	}

	boards, err := a.BoardStore.List()
	if err != nil || len(boards) == 0 {
		return &kanerr.NotInitializedError{Path: a.ProjectRoot}
	}
	return nil
}

// GetAuthor returns the username for authorship fields (card creator, comment author).
func (a *App) GetAuthor() (string, error) {
	return creator.GetAuthor(a.GitClient)
}

// Fatal prints a styled error and exits.
func Fatal(err error) {
	PrintError("%v", err)
	os.Exit(1)
}
