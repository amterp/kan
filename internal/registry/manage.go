package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
)

// State is what kan finds at a registry entry's path.
type State string

const (
	StateOK         State = "ok"
	StateMissing    State = "missing"
	StateUnreadable State = "unreadable"
	StateDuplicate  State = "duplicate board ID"
)

// Status describes one registry entry as found on disk.
type Status struct {
	Entry
	Name   string  `json:"name"`
	State  State   `json:"state"`
	Boards []Board `json:"boards"`
	// Error explains an unreadable entry.
	Error string `json:"error,omitempty"`
	// Duplicates maps each of this project's board IDs that another visible
	// entry also holds to those entries' paths.
	Duplicates map[string][]string `json:"duplicates,omitempty"`
}

// ErrNotRegistered is returned for a path that has no registry entry.
var ErrNotRegistered = errors.New("not in the registry")

// Find returns the registry key for path: the path as given (made
// absolute), or the entry whose symlink-resolved path matches it.
func Find(cfg *model.GlobalConfig, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, ok := cfg.Repos[abs]; ok {
		return abs, nil
	}
	want := resolve(abs)
	for key := range cfg.Repos {
		if resolve(key) == want {
			return key, nil
		}
	}
	return "", fmt.Errorf("%s is %w. Run 'kan registry list --hidden' to see registered paths", path, ErrNotRegistered)
}

func resolve(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return filepath.Clean(path)
}

// Inspect reports the state of every visible entry, plus hidden ones when
// includeHidden is set, sorted by path.
func Inspect(cfg *model.GlobalConfig, includeHidden bool) []Status {
	var statuses []Status
	for _, e := range Entries(cfg) {
		if e.Hidden && !includeHidden {
			continue
		}
		statuses = append(statuses, inspect(cfg, e))
	}
	return statuses
}

func inspect(cfg *model.GlobalConfig, e Entry) Status {
	s := Status{Entry: e, Name: ProjectName(e.Path, e.DataLocation), State: StateOK}

	state, err := locate(e)
	if state != StateOK {
		s.State = state
		if err != nil {
			s.Error = err.Error()
		}
		return s
	}

	boards, err := ReadBoards(e.Path, e.DataLocation)
	if err != nil {
		s.State = StateUnreadable
		s.Error = err.Error()
		return s
	}
	s.Boards = boards

	if e.Hidden {
		return s
	}
	for _, b := range boards {
		if holders := HoldersOf(cfg, b.ID, e.Path); len(holders) > 0 {
			if s.Duplicates == nil {
				s.Duplicates = make(map[string][]string)
			}
			s.Duplicates[b.ID] = holders
			s.State = StateDuplicate
		}
	}
	return s
}

// locate checks that the entry's path and its board folder exist. Missing
// means confirmed gone; any other failure to look is unreadable, never
// missing, so prune can't remove an entry it merely failed to read.
func locate(e Entry) (State, error) {
	if _, err := os.Stat(e.Path); err != nil {
		if os.IsNotExist(err) {
			return StateMissing, nil
		}
		return StateUnreadable, err
	}
	if _, err := os.Stat(config.NewPaths(e.Path, e.DataLocation).BoardsRoot()); err != nil {
		if os.IsNotExist(err) {
			return StateMissing, nil
		}
		return StateUnreadable, err
	}
	return StateOK, nil
}

// SetHidden hides or unhides the entry for path and returns its key.
func SetHidden(cfg *model.GlobalConfig, path string, hidden bool) (string, error) {
	key, err := Find(cfg, path)
	if err != nil {
		return "", err
	}
	repo := cfg.Repos[key]
	repo.Hidden = hidden
	cfg.Repos[key] = repo
	return key, nil
}

// SharedBoards returns, for each board of the project at key, the other
// visible entries holding the same board ID. Unhiding a project with any is
// ambiguous: references to those boards no longer resolve to one copy.
func SharedBoards(cfg *model.GlobalConfig, key string) map[string][]string {
	repo := cfg.Repos[key]
	boards, err := ReadBoards(key, repo.DataLocation)
	if err != nil {
		return nil
	}
	shared := make(map[string][]string)
	for _, b := range boards {
		if holders := HoldersOf(cfg, b.ID, key); len(holders) > 0 {
			shared[b.ID] = holders
		}
	}
	return shared
}

// Remove deletes the entry for path, including settings such as its data
// location, and returns its key.
func Remove(cfg *model.GlobalConfig, path string) (string, error) {
	key, err := Find(cfg, path)
	if err != nil {
		return "", err
	}
	cfg.RemoveRepoConfig(key)
	return key, nil
}

// Prune removes every entry whose state is missing, hidden ones included,
// and returns their paths sorted. With dryRun it only reports them.
func Prune(cfg *model.GlobalConfig, dryRun bool) []string {
	var removed []string
	for _, e := range Entries(cfg) {
		if state, _ := locate(e); state == StateMissing {
			removed = append(removed, e.Path)
		}
	}
	sort.Strings(removed)
	if !dryRun {
		for _, path := range removed {
			cfg.RemoveRepoConfig(path)
		}
	}
	return removed
}
