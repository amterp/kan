package cli

import (
	"fmt"
	"os"

	"github.com/amterp/kan/internal/config"
	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/git"
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
	Paths         *config.Paths
	BoardStore    store.BoardStore
	CardStore     store.CardStore
	Prompter      prompt.Prompter
	InitService   *service.InitService
	BoardService  *service.BoardService
	CardService   *service.CardService
	AliasService  *service.AliasService
	BoardResolver *resolver.BoardResolver
	CardResolver  *resolver.CardResolver
	RepoRoot      string
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

	// Try to get repo root - not all commands require it
	repoRoot, _ := gitClient.GetRepoRoot()

	// Get custom data location if configured
	var dataLocation string
	if repoRoot != "" && globalCfg != nil {
		if repoCfg := globalCfg.GetRepoConfig(repoRoot); repoCfg != nil {
			dataLocation = repoCfg.DataLocation
		}
	}

	paths := config.NewPaths(repoRoot, dataLocation)
	boardStore := store.NewBoardStore(paths)
	cardStore := store.NewCardStore(paths)

	var prompter prompt.Prompter
	if interactive {
		prompter = prompt.NewHuhPrompter()
	} else {
		prompter = &prompt.NoopPrompter{}
	}

	aliasService := service.NewAliasService(cardStore)
	initService := service.NewInitService(gitClient, globalStore)
	boardService := service.NewBoardService(boardStore)
	cardService := service.NewCardService(cardStore, boardStore, aliasService)
	boardResolver := resolver.NewBoardResolver(boardStore, globalStore, prompter, repoRoot)
	cardResolver := resolver.NewCardResolver(cardStore)

	return &App{
		GitClient:     gitClient,
		GlobalStore:   globalStore,
		Paths:         paths,
		BoardStore:    boardStore,
		CardStore:     cardStore,
		Prompter:      prompter,
		InitService:   initService,
		BoardService:  boardService,
		CardService:   cardService,
		AliasService:  aliasService,
		BoardResolver: boardResolver,
		CardResolver:  cardResolver,
		RepoRoot:      repoRoot,
	}, nil
}

// RequireRepo ensures we're in a git repository.
func (a *App) RequireRepo() error {
	if a.RepoRoot == "" {
		return fmt.Errorf("not in a git repository")
	}
	return nil
}

// RequireKan ensures Kan is initialized in the current repo.
func (a *App) RequireKan() error {
	if err := a.RequireRepo(); err != nil {
		return err
	}

	boards, err := a.BoardStore.List()
	if err != nil || len(boards) == 0 {
		return &kanerr.NotInitializedError{Path: a.RepoRoot}
	}
	return nil
}

// GetCreator returns the git username for the card creator field.
func (a *App) GetCreator() string {
	name, err := a.GitClient.GetUserName()
	if err != nil {
		return "unknown"
	}
	return name
}

// Fatal prints an error and exits.
func Fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
