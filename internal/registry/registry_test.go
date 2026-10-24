package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/model"
)

// useTempRoots makes only roots count as temporary for the rest of the test,
// since every project a test creates lives under the real os.TempDir().
func useTempRoots(t *testing.T, roots ...string) {
	t.Helper()
	saved := tempRoots
	tempRoots = func() []string { return roots }
	t.Cleanup(func() { tempRoots = saved })
}

// makeProject creates a project at root with one board per id, under
// dataLocation (default .kan), and returns root.
func makeProject(t *testing.T, root, dataLocation string, boardIDs ...string) string {
	t.Helper()
	if dataLocation == "" {
		dataLocation = ".kan"
	}
	for i, id := range boardIDs {
		dir := filepath.Join(root, dataLocation, "boards", "b"+string(rune('0'+i)))
		if err := os.MkdirAll(filepath.Join(dir, "cards"), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		cfg := "kan_schema = \"board/2\"\nid = \"" + id + "\"\nname = \"b\"\n"
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0644); err != nil {
			t.Fatalf("write board: %v", err)
		}
	}
	return root
}

func resolved(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return dir
}

func TestIsTempPath_ComparesWholeComponents(t *testing.T) {
	useTempRoots(t, "/tmp")
	cases := map[string]bool{
		"/tmp":            true,
		"/tmp/project":    true,
		"/tmp/a/b":        true,
		"/tmpfoo":         false,
		"/tmpfoo/project": false,
		"/home/me/tmp":    false,
	}
	for path, want := range cases {
		if got := IsTempPath(path); got != want {
			t.Errorf("IsTempPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsTempPath_ResolvesSymlinks(t *testing.T) {
	realTemp := resolved(t)
	link := filepath.Join(resolved(t), "link")
	if err := os.Symlink(realTemp, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	useTempRoots(t, link)

	if !IsTempPath(filepath.Join(realTemp, "project")) {
		t.Error("a path under the root's symlink target should count as temp")
	}
}

func TestRegister_TempDirWithoutLocation_RegistersNothing(t *testing.T) {
	temp := resolved(t)
	useTempRoots(t, temp)
	root := makeProject(t, filepath.Join(temp, "scratch"), "", "b_1")
	cfg := &model.GlobalConfig{}

	result := Register(cfg, root, "")

	if result.Registered || len(cfg.Repos) != 0 {
		t.Errorf("temp project should not be registered: %+v, repos %v", result, cfg.Repos)
	}
}

func TestRegister_TempDirWithCustomLocation_RegistersHidden(t *testing.T) {
	temp := resolved(t)
	useTempRoots(t, temp)
	root := makeProject(t, filepath.Join(temp, "scratch"), "kan-data", "b_1")
	cfg := &model.GlobalConfig{}

	result := Register(cfg, root, "kan-data")

	repo := cfg.GetRepoConfig(root)
	if !result.Registered || repo == nil || !repo.Hidden || repo.DataLocation != "kan-data" {
		t.Errorf("want hidden entry with data location, got %+v / %+v", result, repo)
	}
	if result.Message() != "" {
		t.Errorf("a temp registration needs no message, got %q", result.Message())
	}
}

func TestRegister_SecondCopyOfBoard_RegistersHiddenWithMessage(t *testing.T) {
	useTempRoots(t, "/nonexistent-temp-root")
	first := makeProject(t, resolved(t), "", "b_2hH3HWg0E")
	second := makeProject(t, resolved(t), "", "b_2hH3HWg0E")
	cfg := &model.GlobalConfig{}
	if r := Register(cfg, first, ""); r.Hidden {
		t.Fatalf("first copy should be visible: %+v", r)
	}

	result := Register(cfg, second, "")

	if !result.Hidden || !cfg.GetRepoConfig(second).Hidden {
		t.Fatalf("second copy should be hidden: %+v", result)
	}
	want := "board b_2hH3HWg0E is already registered at " + first + "; registered " + second +
		" as hidden. Run \"kan registry unhide " + second + "\" to use this copy instead."
	if result.Message() != want {
		t.Errorf("message:\n got %q\nwant %q", result.Message(), want)
	}
}

func TestRegister_HiddenHolderDoesNotCount(t *testing.T) {
	useTempRoots(t, "/nonexistent-temp-root")
	hiddenCopy := makeProject(t, resolved(t), "", "b_1")
	root := makeProject(t, resolved(t), "", "b_1")
	cfg := &model.GlobalConfig{}
	cfg.SetRepoConfig(hiddenCopy, model.RepoConfig{Hidden: true})

	if result := Register(cfg, root, ""); result.Hidden {
		t.Errorf("a hidden entry must not hide a new registration: %+v", result)
	}
}

func TestRegister_DifferentBoards_Visible(t *testing.T) {
	useTempRoots(t, "/nonexistent-temp-root")
	other := makeProject(t, resolved(t), "", "b_1")
	root := makeProject(t, resolved(t), "", "b_2")
	cfg := &model.GlobalConfig{}
	Register(cfg, other, "")

	if result := Register(cfg, root, ""); result.Hidden || !result.Registered {
		t.Errorf("want visible registration, got %+v", result)
	}
}

func TestProjectName(t *testing.T) {
	root := makeProject(t, resolved(t), "", "b_1")
	if got := ProjectName(root, ""); got != filepath.Base(root) {
		t.Errorf("without a project config, ProjectName = %q, want folder name", got)
	}

	// An older schema still yields its name: the read is raw.
	cfg := "kan_schema = \"project/1\"\nname = \"api\"\n"
	if err := os.WriteFile(filepath.Join(root, ".kan", "config.toml"), []byte(cfg), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := ProjectName(root, ""); got != "api" {
		t.Errorf("ProjectName = %q, want api", got)
	}
}

func TestReadBoards(t *testing.T) {
	root := makeProject(t, resolved(t), "", "b_1", "b_2")
	boards, err := ReadBoards(root, "")
	if err != nil {
		t.Fatalf("ReadBoards: %v", err)
	}
	if len(boards) != 2 || boards[0].ID != "b_1" || boards[1].ID != "b_2" {
		t.Errorf("boards = %+v", boards)
	}
}

// A symlinked spelling of a registered project is that project, not a
// second copy of its boards.
func TestRegister_SymlinkedSpellingOfRegisteredProject(t *testing.T) {
	useTempRoots(t, "/nonexistent-temp-root")
	target := makeProject(t, resolved(t), "", "b_1")
	link := filepath.Join(resolved(t), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cfg := &model.GlobalConfig{}
	Register(cfg, target, "")

	result := Register(cfg, link, "")

	if result.Registered || result.Hidden || len(cfg.Repos) != 1 {
		t.Errorf("want the existing entry kept and nothing added: %+v, repos %v", result, cfg.Repos)
	}
	if holders := HoldersOf(cfg, "b_1", link); len(holders) != 0 {
		t.Errorf("the project itself, through a symlink, should not hold its own board: %v", holders)
	}
}
