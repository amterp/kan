package api

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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
// Auto-migrates board/card schemas and ensures the project config is initialized
// before creating stores, since stores enforce strict schema validation on reads.
func BuildProjectContext(projectRoot, dataLocation, creator string) (*ProjectContext, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf("project root is required")
	}

	// Verify the project path actually exists on disk
	if _, err := os.Stat(projectRoot); err != nil {
		return nil, fmt.Errorf("project path does not exist: %s", projectRoot)
	}

	paths := config.NewPaths(projectRoot, dataLocation)

	// Auto-migrate boards + cards before creating stores (stores reject
	// old schemas on read). Mirrors the startup path in cli/app.go.
	if err := autoMigrateProject(paths, projectRoot); err != nil {
		return nil, err
	}

	boardStore := store.NewBoardStore(paths)
	cardStore := store.NewCardStore(paths)
	projectStore := store.NewProjectStore(paths)

	// Ensure project config exists with ID and current schema.
	// Uses raw file I/O, so it's safe to call before store reads.
	defaultName := filepath.Base(projectRoot)
	if err := projectStore.EnsureInitialized(defaultName); err != nil {
		log.Printf("Warning: failed to initialize project config for %s: %v", projectRoot, err)
	}

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

// autoMigrateProject checks a project's boards and cards for outdated schemas
// and migrates transparently. Returns an error for future versions (user needs
// a newer Kan) or if migration fails.
func autoMigrateProject(paths *config.Paths, projectRoot string) error {
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
		return fmt.Errorf("auto-migration failed for %s: %w", projectRoot, err)
	}

	log.Printf("Auto-migrated project at %s to current schema", projectRoot)
	return nil
}
