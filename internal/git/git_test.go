package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestStatusPorcelain_NoChanges(t *testing.T) {
	dir := initTestRepo(t)
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "add file")

	client := NewClient()
	out, err := client.StatusPorcelain(dir, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty status, got %q", out)
	}
}

func TestStatusPorcelain_WithChanges(t *testing.T) {
	dir := initTestRepo(t)
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	client := NewClient()
	out, err := client.StatusPorcelain(dir, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty status for untracked file")
	}
}

func TestAdd(t *testing.T) {
	dir := initTestRepo(t)
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	client := NewClient()
	if err := client.Add(dir, "file.txt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file is staged
	out, err := client.StatusPorcelain(dir, "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Error("expected file to be staged after Add")
	}
}

func TestCommit(t *testing.T) {
	dir := initTestRepo(t)
	chdir(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	client := NewClient()
	if err := client.Add(dir, "file.txt"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if err := client.Commit(dir, "test commit", "file.txt"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Verify commit exists with correct message
	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(string(out), "test commit") {
		t.Errorf("expected commit message %q in log output %q", "test commit", string(out))
	}
}

func TestCommit_OnlyScopedPaths(t *testing.T) {
	dir := initTestRepo(t)
	chdir(t, dir)

	// Create two files
	kanFile := filepath.Join(dir, ".kan", "board.txt")
	if err := os.MkdirAll(filepath.Dir(kanFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kanFile, []byte("kan content"), 0644); err != nil {
		t.Fatal(err)
	}
	otherFile := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(otherFile, []byte("other content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Stage other.txt manually
	runGit(t, dir, "add", "other.txt")

	client := NewClient()
	if err := client.Add(dir, ".kan"); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if err := client.Commit(dir, "kan only", ".kan"); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// other.txt should still be staged (not committed)
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff failed: %v", err)
	}
	if !strings.Contains(string(out), "other.txt") {
		t.Errorf("expected other.txt to remain staged, got %q", string(out))
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
