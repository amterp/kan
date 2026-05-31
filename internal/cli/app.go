package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/creator"
	"github.com/amterp/kan/internal/discovery"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/git"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/prompt"
	"github.com/amterp/kan/internal/resolver"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
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
	AliasService  *service.AliasService
	HookService   *service.HookService
	BoardResolver *resolver.BoardResolver
	CardResolver  *resolver.CardResolver
	ProjectRoot   string
	// UsingGlobalBoard is true when the App was built via -g (targeting the
	// designated global board). Handlers use it to surface the target in output.
	UsingGlobalBoard bool
}

// AppOptions configures how NewAppWithOptions wires up the App.
type AppOptions struct {
	// Interactive selects the HuhPrompter (vs the NoopPrompter that fails on prompts).
	Interactive bool
	// UseGlobalBoard bypasses cwd discovery and targets the designated global
	// board's project instead (see `kan global set`). Errors if none is set.
	UseGlobalBoard bool
}

// NewApp creates a new App with all dependencies wired up.
// If interactive is false, uses NoopPrompter that fails on prompts.
func NewApp(interactive bool) (*App, error) {
	return NewAppWithOptions(AppOptions{Interactive: interactive})
}

// NewAppWithOptions creates a new App, optionally retargeted at the designated
// global board instead of the project discovered from the cwd.
func NewAppWithOptions(opts AppOptions) (*App, error) {
	interactive := opts.Interactive
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
		// Discover project root (VCS-agnostic)
		result, derr := discovery.DiscoverProject(globalCfg)
		if derr != nil {
			// This is a real error (e.g., global config says path exists but it doesn't)
			return nil, derr
		}

		// If we're in a git worktree, redirect to the main worktree's board
		if result != nil {
			result, derr = discovery.ResolveWorktree(result, gitClient, globalCfg, isWorktreeIndependent)
			if derr != nil {
				return nil, derr
			}
		}

		if result != nil {
			projectRoot = result.ProjectRoot
			dataLocation = result.DataLocation

			if result.ResolvedFromWorktree {
				PrintInfo("Using board from main worktree at %s", result.ProjectRoot)

				// Clean up stale global config entry for the worktree path
				if globalCfg != nil && result.OriginalWorktreeRoot != "" {
					cleanupStaleWorktreeEntry(globalStore, globalCfg, result.OriginalWorktreeRoot)
				}
			}

			// Auto-register unregistered projects (but not worktree paths)
			if !result.WasRegistered && !result.ResolvedFromWorktree && globalCfg != nil {
				registerProject(globalStore, globalCfg, projectRoot, dataLocation)
			}
		}
		// projectRoot may be empty - that's OK, RequireKan() will catch it
	}

	// Auto-migrate project data before creating stores (which do strict
	// version validation on reads).
	if projectRoot != "" {
		migratePaths := config.NewPaths(projectRoot, dataLocation)
		if err := autoMigrateProject(migratePaths); err != nil {
			// Hard error for any migration failure: future-version errors
			// produce specific "upgrade Kan" messages; other failures are
			// more informative than the store-level schema rejection that
			// would follow.
			return nil, err
		}
	}

	paths := config.NewPaths(projectRoot, dataLocation)
	boardStore := store.NewBoardStore(paths)
	cardStore := store.NewCardStore(paths)
	projectStore := store.NewProjectStore(paths)

	// In global mode, fail early with a clear message if the designation has gone
	// stale (project moved/deleted, or board removed) rather than surfacing a
	// generic "not initialized" later.
	if opts.UseGlobalBoard && !boardStore.Exists(globalBoardName) {
		return nil, &kanerr.StaleGlobalBoardError{Board: globalBoardName, Path: prettyPath(projectRoot)}
	}

	// Ensure project config exists with ID (graceful upgrade for older projects)
	if projectRoot != "" {
		defaultName := filepath.Base(projectRoot)
		if err := projectStore.EnsureInitialized(defaultName); err != nil {
			// Non-fatal: log warning but continue
			PrintWarning("failed to initialize project config: %v", err)
		}
	}

	var prompter prompt.Prompter
	if interactive {
		prompter = prompt.NewHuhPrompter()
	} else {
		prompter = &prompt.NoopPrompter{}
	}

	aliasService := service.NewAliasService(cardStore)
	initService := service.NewInitService(globalStore)
	boardService := service.NewBoardService(boardStore, cardStore)
	cardService := service.NewCardService(cardStore, boardStore, aliasService)
	boardResolver := resolver.NewBoardResolver(boardStore, globalStore, prompter, projectRoot)
	if opts.UseGlobalBoard {
		boardResolver.SetPreferredBoard(globalBoardName)
	}
	cardResolver := resolver.NewCardResolver(cardStore)

	// Set up hook service if we have a project root
	var hookService *service.HookService
	if projectRoot != "" {
		hookService = service.NewHookService(projectRoot)
		cardService.SetHookService(hookService)
	}

	return &App{
		GitClient:        gitClient,
		GlobalStore:      globalStore,
		ProjectStore:     projectStore,
		Paths:            paths,
		BoardStore:       boardStore,
		CardStore:        cardStore,
		Prompter:         prompter,
		InitService:      initService,
		BoardService:     boardService,
		CardService:      cardService,
		AliasService:     aliasService,
		HookService:      hookService,
		BoardResolver:    boardResolver,
		CardResolver:     cardResolver,
		ProjectRoot:      projectRoot,
		UsingGlobalBoard: opts.UseGlobalBoard,
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

// registerProject auto-registers a discovered but unregistered project in global config.
func registerProject(globalStore store.GlobalStore, globalCfg *model.GlobalConfig, projectRoot, dataLocation string) {
	projectName := filepath.Base(projectRoot)
	globalCfg.RegisterProject(projectName, projectRoot)

	repoCfg := model.RepoConfig{}
	if dataLocation != "" {
		repoCfg.DataLocation = dataLocation
	}
	globalCfg.SetRepoConfig(projectRoot, repoCfg)

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

// autoMigrateProject checks a project's boards and cards for outdated schema
// versions and migrates transparently. Returns a SchemaVersionError for future
// versions, or a wrapped error if migration fails.
func autoMigrateProject(paths *config.Paths) error {
	svc := service.NewQuietMigrateService(paths)
	plan, err := svc.PlanBoardsOnly()
	if err != nil {
		return fmt.Errorf("failed to check project schema versions: %w", err)
	}

	if !plan.HasChanges() {
		return nil
	}

	if err := plan.FutureVersionError(); err != nil {
		return err
	}

	if err := svc.Execute(plan, false); err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	PrintInfo("Auto-migrated project data to current schema.")
	fmt.Fprintln(os.Stderr, RenderMuted("  Tip: commit the migrated files separately."))
	return nil
}

// PrintGlobalTarget surfaces which global board a command acted on, so a
// mistaken -g designation is immediately visible. No-op outside global mode.
// Writes to stderr (like other status output), so it never pollutes --json.
func (a *App) PrintGlobalTarget(boardName string) {
	if !a.UsingGlobalBoard {
		return
	}
	PrintInfo("global board %q %s", boardName, RenderMuted("("+prettyPath(a.ProjectRoot)+")"))
}

// prettyPath abbreviates the user's home directory to ~ for friendlier output.
func prettyPath(p string) string {
	home, err := os.UserHomeDir()
	// Require a path-segment boundary so /home/alice doesn't match
	// /home/alicewonderland.
	if err == nil && home != "" && (p == home || strings.HasPrefix(p, home+string(os.PathSeparator))) {
		return "~" + p[len(home):]
	}
	return p
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

// CardResolution holds the result of resolving a card along with its board.
type CardResolution struct {
	Card      *model.Card
	BoardName string
	// CrossBoard is true when the card was found via cross-board search
	// (no explicit board flag, no single board, no default). Callers can use
	// this to decide whether to print which board the card was found in.
	CrossBoard bool
	// MultipleBoards is true when the project has more than one board.
	// Used by callers to decide whether to show board context in output.
	MultipleBoards bool
}

// ResolveCardWithBoard resolves both the board and card for commands that take
// a card ID/alias. When no board is specified and there are fewer than
// MaxBoardsForCrossSearch boards, searches across all boards automatically.
func (a *App) ResolveCardWithBoard(explicitBoard, idOrAlias string, interactive bool) (*CardResolution, error) {
	// In global mode (-g), scope strictly to the resolved board (the designated
	// global board, or an explicit -b). Cross-board search would defeat the
	// "-g means this board" contract - a card on another board in the same
	// project must not resolve, and PrintGlobalTarget must report the real
	// designated board, not wherever a stray match was found.
	if a.UsingGlobalBoard {
		boardName, err := a.BoardResolver.Resolve(explicitBoard, false)
		if err != nil {
			return nil, err
		}
		card, cardErr := a.CardResolver.Resolve(boardName, idOrAlias)
		if cardErr != nil {
			return nil, cardErr
		}
		return &CardResolution{Card: card, BoardName: boardName, MultipleBoards: a.hasMultipleBoards()}, nil
	}

	// 1. If explicit board flag provided, use it strictly - no fallback.
	if explicitBoard != "" {
		boardName, err := a.BoardResolver.Resolve(explicitBoard, false)
		if err != nil {
			return nil, err
		}
		card, cardErr := a.CardResolver.Resolve(boardName, idOrAlias)
		if cardErr != nil {
			return nil, cardErr
		}
		multipleBoards := a.hasMultipleBoards()
		return &CardResolution{Card: card, BoardName: boardName, MultipleBoards: multipleBoards}, nil
	}

	// 2. Try non-interactive inference (single board, default_board).
	boardName, err := a.BoardResolver.Resolve("", false)
	if err == nil {
		card, cardErr := a.CardResolver.Resolve(boardName, idOrAlias)
		if cardErr == nil {
			multipleBoards := a.hasMultipleBoards()
			return &CardResolution{Card: card, BoardName: boardName, MultipleBoards: multipleBoards}, nil
		}
		// Card not found on the inferred board. If there are multiple boards,
		// fall through to cross-board search rather than failing - the inferred
		// board is a convenience default, not an explicit user choice.
		if !kanerr.IsNotFound(cardErr) {
			return nil, cardErr
		}
	}

	// 3. Inference failed or card wasn't on the inferred board - try cross-board search
	boards, listErr := a.BoardStore.List()
	if listErr != nil {
		return nil, listErr
	}
	multipleBoards := len(boards) > 1

	if len(boards) >= resolver.MaxBoardsForCrossSearch {
		// Too many boards for cross-board search, fall back to picker
		if !interactive {
			return nil, fmt.Errorf("multiple boards exist; specify with -b or set default_board in config")
		}
		boardName, err = a.Prompter.Select("Select board", boards)
		if err != nil {
			return nil, err
		}
		card, cardErr := a.CardResolver.Resolve(boardName, idOrAlias)
		if cardErr != nil {
			return nil, cardErr
		}
		return &CardResolution{Card: card, BoardName: boardName, MultipleBoards: multipleBoards}, nil
	}

	// Cross-board search
	matches, err := a.CardResolver.ResolveAcrossBoards(boards, idOrAlias)
	if err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, kanerr.CardNotFound(idOrAlias)
	case 1:
		return &CardResolution{
			Card:           matches[0].Card,
			BoardName:      matches[0].BoardName,
			CrossBoard:     true,
			MultipleBoards: multipleBoards,
		}, nil
	default:
		if !interactive {
			boardNames := make([]string, len(matches))
			for i, m := range matches {
				boardNames[i] = m.BoardName
			}
			return nil, fmt.Errorf("card %q found in multiple boards (%s); specify with -b",
				idOrAlias, strings.Join(boardNames, ", "))
		}
		matchBoards := make([]string, len(matches))
		for i, m := range matches {
			matchBoards[i] = m.BoardName
		}
		selected, selectErr := a.Prompter.Select("Card found in multiple boards - select one", matchBoards)
		if selectErr != nil {
			return nil, selectErr
		}
		for _, m := range matches {
			if m.BoardName == selected {
				return &CardResolution{
					Card:           m.Card,
					BoardName:      m.BoardName,
					CrossBoard:     true,
					MultipleBoards: multipleBoards,
				}, nil
			}
		}
		return nil, fmt.Errorf("internal error: selected board not in matches")
	}
}

// hasMultipleBoards returns true if more than one board exists.
func (a *App) hasMultipleBoards() bool {
	boards, err := a.BoardStore.List()
	if err != nil {
		return false
	}
	return len(boards) > 1
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
