package cli

import (
	"os"
	"path/filepath"

	"github.com/amterp/kan/internal/git"
	"github.com/amterp/kan/internal/gitdriver"
)

// kanExePath returns the absolute path to the running kan binary, for embedding
// in the merge-driver git config. Symlinks are resolved so the stored command
// keeps working if the symlink is later repointed.
func kanExePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

// driverPaths gathers the inputs gitdriver needs for a project rooted at
// projectRoot whose .kan dir is at kanRoot. ok is false (with no error) when the
// project isn't inside a git repo - the merge driver simply doesn't apply.
func driverPaths(gitClient *git.Client, projectRoot, kanRoot string) (repoRoot, kanRel, kanExe string, ok bool, err error) {
	if projectRoot == "" || gitClient == nil || !gitClient.IsRepoAt(projectRoot) {
		return "", "", "", false, nil
	}
	repoRoot, err = gitClient.GetRepoRootAt(projectRoot)
	if err != nil {
		return "", "", "", false, nil
	}
	kanRel, err = relResolved(repoRoot, kanRoot)
	if err != nil {
		return "", "", "", false, err
	}
	kanExe, err = kanExePath()
	if err != nil {
		return "", "", "", false, err
	}
	return repoRoot, kanRel, kanExe, true, nil
}

// relResolved is filepath.Rel with both operands symlink-resolved first. Git
// reports its repo root in resolved form (e.g. /private/var on macOS) while
// discovery and os.Getwd yield the unresolved symlink path (/var); a naive Rel
// between the two mismatched forms produces a bogus "../../.." path.
func relResolved(base, target string) (string, error) {
	if r, err := filepath.EvalSymlinks(base); err == nil {
		base = r
	}
	if r, err := filepath.EvalSymlinks(target); err == nil {
		target = r
	}
	return filepath.Rel(base, target)
}

// mergeDriverEnabled resolves the opt-out setting for this app's project by
// loading the project and global configs.
func (a *App) mergeDriverEnabled() bool {
	var projMD *bool
	if a.ProjectStore != nil {
		if pc, err := a.ProjectStore.Load(); err == nil && pc != nil {
			projMD = pc.MergeDriver
		}
	}
	var globMD *bool
	if a.GlobalStore != nil {
		if gc, err := a.GlobalStore.Load(); err == nil && gc != nil {
			globMD = gc.MergeDriver
		}
	}
	return mergeDriverEnabled(projMD, globMD)
}

// mergeDriverEnabled resolves the opt-out setting: a per-project value wins over
// the global value, and an unset value defaults to enabled (opt-out).
func mergeDriverEnabled(projectMD, globalMD *bool) bool {
	if projectMD != nil {
		return *projectMD
	}
	if globalMD != nil {
		return *globalMD
	}
	return true
}

// ensureMergeDriver is the startup auto-heal hook. It is heal-only: when enabled
// AND the repo already carries the committed .gitattributes opt-in, it registers
// or repairs this clone's local driver command (untracked git config) and tells
// the user once. It never writes the tracked .gitattributes itself - creating
// that opt-in is reserved for explicit actions (kan init, kan config
// merge-driver on, kan doctor --fix), so a bare command like `kan list` never
// modifies a tracked file. When disabled, or for a repo that never opted in, it
// does nothing. Entirely best-effort: any problem is swallowed so a normal
// command never fails over merge-driver setup.
func ensureMergeDriver(gitClient *git.Client, projectRoot, kanRoot string, enabled bool) {
	if !enabled {
		return
	}
	repoRoot, kanRel, kanExe, ok, err := driverPaths(gitClient, projectRoot, kanRoot)
	if !ok || err != nil {
		return
	}
	if !gitdriver.OptedIn(repoRoot, kanRel) {
		return
	}
	wrote, err := gitdriver.EnsureConfig(gitClient, repoRoot, kanExe)
	if err != nil {
		return
	}
	if wrote {
		PrintInfo("Registered the Kan git merge driver in this repo's local git config (auto-resolves card conflicts). Disable with `kan config merge-driver off`.")
	}
}
