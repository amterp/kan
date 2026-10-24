package service

import (
	"slices"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/store"
)

type ReadOnlyProject struct {
	Boards  *store.FileBoardStore
	Cards   *store.FileCardStore
	Project *store.FileProjectStore
}

func OpenProjectForReading(paths *config.Paths) (*ReadOnlyProject, error) {
	var overlay *store.Overlay
	if paths.ProjectRoot() != "" {
		var err error
		overlay, err = NewQuietMigrateService(paths).LoadProjectOverlay()
		if err != nil {
			return nil, err
		}
	}
	return &ReadOnlyProject{
		Boards: store.NewReadOnlyBoardStore(paths, overlay), Cards: store.NewReadOnlyCardStore(paths, overlay),
		Project: store.NewReadOnlyProjectStore(paths, overlay),
	}, nil
}

func NewElsewhere(paths *config.Paths, boards store.BoardStore, project store.ProjectStore, cfg *model.GlobalConfig) *Elsewhere {
	e := &Elsewhere{
		Locator: registry.NewLocator(cfg, paths, boards),
		Open: func(root, dataLocation string) (store.BoardStore, store.CardStore, error) {
			p, err := OpenProjectForReading(config.NewPaths(root, dataLocation))
			if err != nil {
				return nil, nil, err
			}
			return p.Boards, p.Cards, nil
		},
	}
	if cfg != nil {
		e.Projects = registry.Visible(cfg)
	}
	if paths.ProjectRoot() != "" && !slices.ContainsFunc(e.Projects, func(p registry.Entry) bool { return registry.SamePath(p.Path, paths.ProjectRoot()) }) {
		e.Projects = append(e.Projects, registry.Entry{Path: paths.ProjectRoot(), DataLocation: paths.DataLocation()})
	}
	if project != nil {
		if stored, err := project.Load(); err == nil {
			e.LinkedBoards = stored.LinkedBoards
		}
	}
	return e
}
