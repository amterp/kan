package api

import (
	"fmt"
	"os"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
)

// ProjectContext bundles all per-project dependencies needed by the HTTP handlers.
// The Handler holds one of these and can swap it out on project switch.
type ProjectContext struct {
	Paths        *config.Paths
	BoardStore   store.BoardStore
	CardStore    store.CardStore
	ProjectStore store.ProjectStore
	CardService  *service.CardService
	BoardService *service.BoardService
	Creator      string
	ProjectRoot  string
}

// BuildProjectContext creates a fully-wired ProjectContext from a project root path
// and optional data location override (empty string means default .kan/).
//
// This is a pure construction function — it validates the path exists and wires up
// stores/services but does not perform any disk writes. Callers are responsible for
// any initialization (e.g., EnsureInitialized) before or after calling this.
func BuildProjectContext(projectRoot, dataLocation, creator string) (*ProjectContext, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf("project root is required")
	}

	// Verify the project path actually exists on disk
	if _, err := os.Stat(projectRoot); err != nil {
		return nil, fmt.Errorf("project path does not exist: %s", projectRoot)
	}

	paths := config.NewPaths(projectRoot, dataLocation)

	boardStore := store.NewBoardStore(paths)
	cardStore := store.NewCardStore(paths)
	projectStore := store.NewProjectStore(paths)

	aliasService := service.NewAliasService(cardStore)
	boardService := service.NewBoardService(boardStore, cardStore)
	cardService := service.NewCardService(cardStore, boardStore, aliasService)

	// Set up hook service for pattern hooks
	hookService := service.NewHookService(projectRoot)
	cardService.SetHookService(hookService)

	return &ProjectContext{
		Paths:        paths,
		BoardStore:   boardStore,
		CardStore:    cardStore,
		ProjectStore: projectStore,
		CardService:  cardService,
		BoardService: boardService,
		Creator:      creator,
		ProjectRoot:  projectRoot,
	}, nil
}
