// Package gitdriver installs and inspects the git merge driver that lets Kan
// resolve card-file conflicts semantically (see internal/merge).
//
// Setup has two halves with a deliberate consent split:
//
//   - .gitattributes (committed, lives at the repo root): routes card files
//     through "merge=kan". This is the *repo* opting in, and it travels to every
//     clone via git.
//   - merge.kan.driver (repo-local git config, never committed): the actual
//     command git runs. Git refuses to load driver commands from a cloned repo
//     for security, so each clone must register it locally. Kan does this for
//     the user (on init, on doctor --fix, and by auto-healing on startup when
//     the committed opt-in is present).
package gitdriver

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/amterp/kan/internal/git"
)

const (
	driverName      = "kan"
	configKeyDriver = "merge.kan.driver"
	configKeyName   = "merge.kan.name"
	driverHelpName  = "Kan card-aware merge"

	attributesComment = "# Kan: resolve card-file merge conflicts semantically (see `kan doctor`)"
)

// Result reports what Install changed, so callers can tailor their messaging.
type Result struct {
	WroteAttributes bool
	WroteConfig     bool
}

// Status describes the current state of the merge-driver setup for a project.
type Status struct {
	// OptedIn is true when .gitattributes routes card files through the driver.
	OptedIn bool
	// ConfigPresent is true when a local merge.kan.driver command is registered.
	ConfigPresent bool
	// ConfigUpToDate is true when the registered command matches what we'd
	// install for the current binary (catches a moved/renamed kan binary).
	ConfigUpToDate bool
}

// Install opts the repo into the kan merge driver and registers the local
// command that honors it. Idempotent: re-running only writes what's missing or
// stale. kanRel is the .kan directory relative to repoRoot; kanExe is the
// absolute path to the kan binary.
func Install(g *git.Client, repoRoot, kanRel, kanExe string) (Result, error) {
	var res Result

	wroteAttrs, err := ensureAttributes(repoRoot, kanRel)
	if err != nil {
		return res, err
	}
	res.WroteAttributes = wroteAttrs

	wroteCfg, err := EnsureConfig(g, repoRoot, kanExe)
	if err != nil {
		return res, err
	}
	res.WroteConfig = wroteCfg
	return res, nil
}

// EnsureConfig registers (or repairs) only the local driver command, without
// touching .gitattributes. This is the auto-heal path: a freshly cloned repo
// already carries the committed opt-in, and the local command just needs to be
// pointed at this machine's kan binary. Returns whether anything changed.
func EnsureConfig(g *git.Client, repoRoot, kanExe string) (bool, error) {
	want := driverCommand(kanExe)
	if g.GetConfigLocal(repoRoot, configKeyDriver) == want {
		return false, nil
	}
	if err := g.SetConfigLocal(repoRoot, configKeyName, driverHelpName); err != nil {
		return false, err
	}
	if err := g.SetConfigLocal(repoRoot, configKeyDriver, want); err != nil {
		return false, err
	}
	return true, nil
}

// Uninstall removes the merge driver: it strips the .gitattributes opt-in line
// and unsets the local driver config. Idempotent. Used when a user opts a
// project out (kan config merge-driver off).
func Uninstall(g *git.Client, repoRoot, kanRel string) error {
	if err := removeAttributesLine(repoRoot, kanRel); err != nil {
		return err
	}
	if err := g.UnsetConfigLocal(repoRoot, configKeyDriver); err != nil {
		return err
	}
	return g.UnsetConfigLocal(repoRoot, configKeyName)
}

func removeAttributesLine(repoRoot, kanRel string) error {
	path := filepath.Join(repoRoot, ".gitattributes")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	want := attributesLine(kanRel)
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == want || trimmed == attributesComment {
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n")

	// If nothing but whitespace remains, remove the file entirely.
	if strings.TrimSpace(out) == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// GetStatus inspects the current setup for diagnostics (kan doctor).
func GetStatus(g *git.Client, repoRoot, kanRel, kanExe string) Status {
	cmd := g.GetConfigLocal(repoRoot, configKeyDriver)
	return Status{
		OptedIn:        OptedIn(repoRoot, kanRel),
		ConfigPresent:  cmd != "",
		ConfigUpToDate: cmd == driverCommand(kanExe),
	}
}

// OptedIn reports whether .gitattributes already routes this project's card
// files through the kan merge driver - the committed opt-in signal that gates
// auto-heal (Kan won't touch git config for a repo that never opted in).
func OptedIn(repoRoot, kanRel string) bool {
	data, err := os.ReadFile(filepath.Join(repoRoot, ".gitattributes"))
	if err != nil {
		return false
	}
	return hasLine(data, attributesLine(kanRel))
}

func ensureAttributes(repoRoot, kanRel string) (bool, error) {
	path := filepath.Join(repoRoot, ".gitattributes")
	line := attributesLine(kanRel)

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if hasLine(data, line) {
		return false, nil
	}

	var buf bytes.Buffer
	buf.Write(data)
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteString(attributesComment + "\n")
	buf.WriteString(line + "\n")
	return true, os.WriteFile(path, buf.Bytes(), 0o644)
}

// attributesLine is the full .gitattributes entry for a project's card files.
func attributesLine(kanRel string) string {
	return attributesPattern(kanRel) + " merge=" + driverName
}

// attributesPattern returns the glob (relative to the repo root, forward
// slashes) matching every board's card JSON files under the .kan directory.
func attributesPattern(kanRel string) string {
	rel := filepath.ToSlash(filepath.Clean(kanRel))
	if rel == "." || rel == "" {
		return "boards/*/cards/*.json"
	}
	return rel + "/boards/*/cards/*.json"
}

// driverCommand is the value stored in merge.kan.driver. Git runs it through a
// shell and substitutes the %-placeholders, so the binary path is quoted to
// tolerate spaces.
func driverCommand(kanExe string) string {
	return `"` + kanExe + `" merge-driver %O %A %B %P %L`
}

func hasLine(data []byte, line string) bool {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == line {
			return true
		}
	}
	return false
}
