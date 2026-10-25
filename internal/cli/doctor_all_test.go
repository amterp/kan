package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/util"
)

func TestDoctorAllFlags(t *testing.T) {
	for _, args := range [][]string{{"doctor", "--all", "-b", "main"}, {"doctor", "-g", "--all"}} {
		if err := buildRootCmd().RootCmd.ParseOrError(args); err == nil {
			t.Fatalf("accepted conflicting flags: %v", args)
		}
	}
	ctx := buildRootCmd()
	if err := ctx.RootCmd.ParseOrError([]string{"doctor", "--all", "--fix", "--json"}); err != nil || !*ctx.DoctorAll || !*ctx.DoctorFix {
		t.Fatalf("doctor --all did not parse: %v", err)
	}
}

func TestDoctor_RegisteredLinkUsesElsewhere(t *testing.T) {
	root := openFixtureProject(t, "v15")
	api := otherProject(t, "v15", "b_api")
	registerProjects(t, []string{api})
	app := newReadApp(t)
	card, err := store.NewCardStore(app.Paths).Get("main", "card-abc")
	if err != nil {
		t.Fatal(err)
	}
	card.BlockedBy = []string{"b_api:card-abc"}
	card.Related = nil
	if err := store.NewCardStore(app.Paths).Update("main", card); err != nil {
		t.Fatal(err)
	}
	before, remoteBefore := snapshotProject(t, root), snapshotProject(t, api)
	out := captureStdout(t, func() { runDoctor("main", false, false, true, false, false) })
	var report service.DiagnosticReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	for _, issue := range report.Issues {
		if issue.Code == service.CodeUnresolvedLink || issue.Code == service.CodeAbsentLink {
			t.Fatalf("valid registered link was not resolved: %+v", report.Issues)
		}
	}
	assertProjectUnchanged(t, before, snapshotProject(t, root))
	assertProjectUnchanged(t, remoteBefore, snapshotProject(t, api))
}

func TestDoctorAll_FixesOnlyCurrentProjectAndReportsSkips(t *testing.T) {
	root := openFixtureProject(t, "v15")
	api := otherProject(t, "v15", "b_api")
	hidden := otherProject(t, "v15", "b_hidden")
	missing := filepath.Join(t.TempDir(), "missing")
	registerProjects(t, []string{api, missing}, hidden)
	app, err := NewAppWithOptions(AppOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		paths *config.Paths
		ref   string
	}{
		{app.Paths, "b_api:missing"}, {config.NewPaths(api, ""), "missing"},
	} {
		cards := store.NewCardStore(target.paths)
		card, err := cards.Get("main", "card-abc")
		if err != nil {
			t.Fatal(err)
		}
		card.BlockedBy = []string{target.ref}
		card.Related = nil
		card.Position = util.PositionInitial(1)[0]
		if err := cards.Update("main", card); err != nil {
			t.Fatal(err)
		}
	}
	remoteBefore, hiddenBefore := snapshotProject(t, api), snapshotProject(t, hidden)
	report, skipped, err := doctorAll(app, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Boards) != 2 || len(skipped) != 1 || skipped[0].Path != missing || report.Summary.Fixed != 1 {
		t.Fatalf("unexpected all report: %+v, %+v", report, skipped)
	}
	for _, issue := range report.Issues {
		if issue.Code == service.CodeAbsentLink && (issue.ProjectPath != api || issue.Fixable) {
			t.Fatalf("foreign issue could be fixed in %s: %+v", root, issue)
		}
	}
	card, err := app.CardStore.Get("main", "card-abc")
	if err != nil || len(card.BlockedBy) != 0 {
		t.Fatalf("current link not fixed: %+v, %v", card, err)
	}
	assertProjectUnchanged(t, remoteBefore, snapshotProject(t, api))
	assertProjectUnchanged(t, hiddenBefore, snapshotProject(t, hidden))
	foreign := service.Issue{ProjectPath: api, Board: "main", CardID: "card-abc", Code: service.CodeInvalidParentRef, Fixable: true}
	before := snapshotProject(t, root)
	if _, err := newDoctorService(app, app).Fix(&service.DiagnosticReport{Issues: []service.Issue{foreign}}); err != nil {
		t.Fatal(err)
	}
	assertProjectUnchanged(t, before, snapshotProject(t, root))
}

func TestDoctorAll_OutsideProjectDuplicateIDsAndOldSchemas(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(resolvedTempDir(t))
	api := otherProject(t, "v2", "b_api")
	clone := otherProject(t, "v15", "b_api")
	registerProjects(t, []string{api, clone})
	before := snapshotProject(t, api)
	app := newReadApp(t)
	report, skipped, err := doctorAll(app, true)
	if err != nil || len(skipped) != 0 || len(report.Boards) != 2 || report.Summary.Fixed != 0 {
		t.Fatalf("outside-project doctor failed: %+v, %+v, %v", report, skipped, err)
	}
	duplicates := 0
	for _, issue := range report.Issues {
		if issue.Code == service.CodeDuplicateBoardID {
			duplicates++
			if issue.Fixable || !strings.Contains(issue.Message, api) || !strings.Contains(issue.Message, clone) {
				t.Fatalf("duplicate lacks holders: %+v", issue)
			}
		}
	}
	if duplicates != 1 {
		t.Fatalf("expected one duplicate warning: %+v", report.Issues)
	}
	assertProjectUnchanged(t, before, snapshotProject(t, api))
	if _, err := os.Stat(filepath.Join(app.ProjectRoot, ".kan")); !os.IsNotExist(err) {
		t.Fatalf("created local project: %v", err)
	}
}

// Run from a symlinked path, the current project is still the one --fix
// repairs.
func TestDoctorAll_FixesCurrentProjectReachedThroughSymlink(t *testing.T) {
	root := openFixtureProject(t, "v15")
	link := filepath.Join(resolvedTempDir(t), "current-link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Chdir(link)
	app, err := NewAppWithOptions(AppOptions{})
	if err != nil {
		t.Fatal(err)
	}
	card, err := app.CardStore.Get("main", "card-abc")
	if err != nil {
		t.Fatal(err)
	}
	card.BlockedBy = []string{"missing"}
	card.Related = nil
	if err := app.CardStore.Update("main", card); err != nil {
		t.Fatal(err)
	}

	report, _, err := doctorAll(app, true)
	if err != nil {
		t.Fatal(err)
	}
	card, err = app.CardStore.Get("main", "card-abc")
	if err != nil || len(card.BlockedBy) != 0 || report.Summary.Fixed == 0 {
		t.Fatalf("the current project's link was not fixed: fixed %d, blocked_by %v, %v", report.Summary.Fixed, card.BlockedBy, err)
	}
}
