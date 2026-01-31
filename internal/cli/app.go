package cli

import (
	"fmt"
	"os"
	"path/filepath"

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
		fmt.Fprintf(os.Stderr, "Warning: failed to load global config: %v\n", err)
		globalCfg = nil
	}

	// Discover project root (VCS-agnostic)
	var projectRoot, dataLocation string
	result, err := discovery.DiscoverProject(globalCfg)
	if err != nil {
		// This is a real error (e.g., global config says path exists but it doesn't)
		return nil, err
	}
	if result != nil {
		projectRoot = result.ProjectRoot
		dataLocation = result.DataLocation

		// Auto-register unregistered projects
		if !result.WasRegistered && globalCfg != nil {
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
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize project config: %v\n", err)
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

// Fatal prints an error and exits.
func Fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
