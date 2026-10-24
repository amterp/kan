// Package registry holds the rules for the projects kan knows on this machine:
// the [repos] table of the global config. An entry lets discovery find a board
// at a custom data location, and, unless it is hidden, offers the project
// beyond its own directory (the web UI's switcher, queries across projects).
//
// Everything here reads project files raw, without schema validation or
// migration, and never writes inside a project.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
)

// Entry is one project in the registry.
type Entry struct {
	Path         string `json:"path"`
	DataLocation string `json:"data_location,omitempty"`
	Hidden       bool   `json:"hidden"`
}

// Board is a board found in a project.
type Board struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Entries returns every registry entry, sorted by path.
func Entries(cfg *model.GlobalConfig) []Entry {
	entries := make([]Entry, 0, len(cfg.Repos))
	for path, repo := range cfg.Repos {
		entries = append(entries, Entry{Path: path, DataLocation: repo.DataLocation, Hidden: repo.Hidden})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

// Visible returns the entries that are not hidden, sorted by path.
func Visible(cfg *model.GlobalConfig) []Entry {
	var visible []Entry
	for _, e := range Entries(cfg) {
		if !e.Hidden {
			visible = append(visible, e)
		}
	}
	return visible
}

// tempRoots lists the directories whose contents count as temporary. Tests
// substitute it, because their own projects live under os.TempDir().
var tempRoots = func() []string {
	return []string{os.TempDir(), "/tmp", "/private/tmp"}
}

// IsTempPath reports whether path is inside a temp directory. Components are
// compared whole, so /tmpfoo is not inside /tmp, and symlinks are resolved on
// both sides, so /var/folders/... matches /private/var/folders/... on macOS.
func IsTempPath(path string) bool {
	for _, p := range pathForms(path) {
		for _, root := range tempRoots() {
			for _, r := range pathForms(root) {
				if isWithin(p, r) {
					return true
				}
			}
		}
	}
	return false
}

// pathForms returns path as given and, if different, with symlinks
// resolved. Comparing both catches a match even when only one side of a
// comparison exists on disk.
func pathForms(path string) []string {
	forms := []string{filepath.Clean(path)}
	if c := canonical(path); c != forms[0] {
		forms = append(forms, c)
	}
	return forms
}

// samePath reports whether a and b name the same directory, following
// symlinks, so /link/project and /real/project are one project.
func samePath(a, b string) bool {
	return canonical(a) == canonical(b)
}

// canonical resolves symlinks in path, or cleans it when it can't be
// resolved (for example, because it no longer exists).
func canonical(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func isWithin(path, root string) bool {
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	return strings.HasPrefix(path, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))
}

// ReadBoards lists the boards under a project's data location with their IDs.
// A board config that can't be decoded is an error; one without an ID is
// listed with an empty ID.
func ReadBoards(root, dataLocation string) ([]Board, error) {
	paths := config.NewPaths(root, dataLocation)
	dirs, err := os.ReadDir(paths.BoardsRoot())
	if err != nil {
		return nil, err
	}
	var boards []Board
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		data, err := os.ReadFile(paths.BoardConfigPath(d.Name()))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var header struct {
			ID string `toml:"id"`
		}
		if _, err := toml.Decode(string(data), &header); err != nil {
			return nil, fmt.Errorf("invalid board config %s: %w", paths.BoardConfigPath(d.Name()), err)
		}
		boards = append(boards, Board{Name: d.Name(), ID: header.ID})
	}
	return boards, nil
}

// ProjectName returns the name in a project's config, or the project
// folder's name when the config is missing, unreadable or has none.
func ProjectName(root, dataLocation string) string {
	data, err := os.ReadFile(config.NewPaths(root, dataLocation).ProjectConfigPath())
	if err == nil {
		var header struct {
			Name string `toml:"name"`
		}
		if _, err := toml.Decode(string(data), &header); err == nil && header.Name != "" {
			return header.Name
		}
	}
	return filepath.Base(root)
}

// HoldersOf returns the visible entries other than exclude whose project has
// a board with boardID, sorted by path. Hidden entries never count.
func HoldersOf(cfg *model.GlobalConfig, boardID, exclude string) []string {
	if boardID == "" {
		return nil
	}
	var holders []string
	for _, e := range Visible(cfg) {
		if samePath(e.Path, exclude) {
			continue
		}
		boards, err := ReadBoards(e.Path, e.DataLocation)
		if err != nil {
			continue
		}
		for _, b := range boards {
			if b.ID == boardID {
				holders = append(holders, e.Path)
				break
			}
		}
	}
	return holders
}

// Result says what Register did.
type Result struct {
	// Registered is false when the project was left out of the registry.
	Registered bool
	Hidden     bool
	// BoardID and HeldBy are set when the project was hidden because a
	// visible entry at HeldBy already holds one of its boards.
	BoardID string
	HeldBy  string
	Path    string
}

// Message explains a registration the user should know about, or is empty.
func (r Result) Message() string {
	if r.HeldBy == "" {
		return ""
	}
	return fmt.Sprintf("board %s is already registered at %s; registered %s as hidden. "+
		"Run \"kan registry unhide %s\" to use this copy instead.", r.BoardID, r.HeldBy, r.Path, r.Path)
}

// Register adds a project to cfg, keeping one-off copies out of the way:
//   - Inside a temp directory, it registers nothing, since discovery finds
//     .kan/ by walking up from the working directory. A custom data location
//     can't be found that way, so it registers that project hidden.
//   - If a visible entry at another path already holds one of the project's
//     board IDs, it registers the project hidden.
//
// Otherwise the project is registered visible.
func Register(cfg *model.GlobalConfig, root, dataLocation string) Result {
	result := Result{Path: root}
	// The same project reached through a symlink is already registered;
	// a second entry for it would make its boards look like two copies.
	for path := range cfg.Repos {
		if samePath(path, root) {
			result.Path = path
			return result
		}
	}
	if IsTempPath(root) {
		if dataLocation == "" {
			return result
		}
		result.Hidden = true
	}

	if !result.Hidden {
		boards, _ := ReadBoards(root, dataLocation)
		for _, b := range boards {
			if holders := HoldersOf(cfg, b.ID, root); len(holders) > 0 {
				result.Hidden = true
				result.BoardID = b.ID
				result.HeldBy = holders[0]
				break
			}
		}
	}

	cfg.SetRepoConfig(root, model.RepoConfig{DataLocation: dataLocation, Hidden: result.Hidden})
	result.Registered = true
	return result
}
