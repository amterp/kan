package cli

import (
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
}

// NewApp creates a new App with all dependencies wired up.
// If interactive is false, uses NoopPrompter that fails on prompts.
func NewApp(interactive bool) (*App, error) {
	gitClient := git.NewClient()
	globalStore := store.NewGlobalStore()

	// Load global config with warnings (don't silently ignore errors)
	globalCfg, err := globalStore.Load()
	if err != nil {
		PrintWarning("failed to load global config: %v", err)
		globalCfg = nil
	}

	// Discover project root (VCS-agnostic)
	var projectRoot, dataLocation string
	result, err := discovery.DiscoverProject(globalCfg)
	if err != nil {
		// This is a real error (e.g., global config says path exists but it doesn't)
		return nil, err
	}

	// If we're in a git worktree, redirect to the main worktree's board
	if result != nil {
		result, err = discovery.ResolveWorktree(result, gitClient, globalCfg, isWorktreeIndependent)
		if err != nil {
			return nil, err
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

	paths := config.NewPaths(projectRoot, dataLocation)
	boardStore := store.NewBoardStore(paths)
	cardStore := store.NewCardStore(paths)
	projectStore := store.NewProjectStore(paths)

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
	cardResolver := resolver.NewCardResolver(cardStore)

	// Set up hook service if we have a project root
	var hookService *service.HookService
	if projectRoot != "" {
		hookService = service.NewHookService(projectRoot)
		cardService.SetHookService(hookService)
	}

	return &App{
		GitClient:     gitClient,
		GlobalStore:   globalStore,
		ProjectStore:  projectStore,
		Paths:         paths,
		BoardStore:    boardStore,
		CardStore:     cardStore,
		Prompter:      prompter,
		InitService:   initService,
		BoardService:  boardService,
		CardService:   cardService,
		AliasService:  aliasService,
		HookService:   hookService,
		BoardResolver: boardResolver,
		CardResolver:  cardResolver,
		ProjectRoot:   projectRoot,
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
