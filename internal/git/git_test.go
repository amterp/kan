package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsWorktree_MainRepo(t *testing.T) {
	mainDir := initTestRepo(t)
	chdir(t, mainDir)

	client := NewClient()
	if client.IsWorktree() {
		t.Error("expected IsWorktree()=false in main repo")
	}
}

func TestIsWorktree_InWorktree(t *testing.T) {
	mainDir := initTestRepo(t)
	wtDir := createWorktree(t, mainDir, "test-branch")
	chdir(t, wtDir)

	client := NewClient()
	if !client.IsWorktree() {
		t.Error("expected IsWorktree()=true in worktree")
	}
}

func TestIsWorktree_NotARepo(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	client := NewClient()
	if client.IsWorktree() {
		t.Error("expected IsWorktree()=false outside a git repo")
	}
}

func TestGetMainWorktreeRoot(t *testing.T) {
	mainDir := initTestRepo(t)
	wtDir := createWorktree(t, mainDir, "test-branch")
	chdir(t, wtDir)

	client := NewClient()
	root, err := client.GetMainWorktreeRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve symlinks for macOS /private/tmp vs /tmp
	expectedMain, _ := filepath.EvalSymlinks(mainDir)
	actualRoot, _ := filepath.EvalSymlinks(root)

	if actualRoot != expectedMain {
		t.Errorf("expected main root %q, got %q", expectedMain, actualRoot)
	}
}

func TestGetMainWorktreeRoot_NotWorktree(t *testing.T) {
	mainDir := initTestRepo(t)
	chdir(t, mainDir)

	client := NewClient()
	_, err := client.GetMainWorktreeRoot()
	if err == nil {
		t.Error("expected error when not in a worktree")
	}
}

// --- helpers ---

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")
	return dir
}

func createWorktree(t *testing.T, mainDir, branch string) string {
	t.Helper()
	wtDir := filepath.Join(t.TempDir(), "worktree")
	runGit(t, mainDir, "worktree", "add", wtDir, "-b", branch)
	t.Cleanup(func() {
		runGit(t, mainDir, "worktree", "remove", "--force", wtDir)
	})
	return wtDir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}
