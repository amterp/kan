package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/service"
)

type doctorAllOutput struct {
	*service.DiagnosticReport
	Skipped []skippedBoard `json:"skipped"`
}

func runDoctorAll(app *App, fix, dryRun, jsonOutput bool) {
	report, skipped, err := doctorAll(app, fix)
	if err != nil {
		Fatal(err)
	}
	if jsonOutput {
		if err := printJson(doctorAllOutput{DiagnosticReport: report, Skipped: skipped}); err != nil {
			Fatal(err)
		}
	} else {
		printDoctorReport(report, fix, dryRun)
		printSkipped(skipped)
	}
	if report.HasErrors() {
		os.Exit(1)
	}
}

func doctorAll(app *App, fix bool) (*service.DiagnosticReport, []skippedBoard, error) {
	targets, skipped := app.boardsForAll()
	if skipped == nil {
		skipped = []skippedBoard{}
	}
	type selection struct {
		project *App
		names   []string
	}
	var projects []*selection
	byProject := map[*App]*selection{}
	for _, target := range targets {
		selected := byProject[target.Project]
		if selected == nil {
			selected = &selection{project: target.Project}
			byProject[target.Project] = selected
			projects = append(projects, selected)
		}
		selected.names = append(selected.names, target.Name)
	}
	report := &service.DiagnosticReport{Boards: []service.BoardDiagnostic{}, Issues: []service.Issue{}}
	for i, selected := range projects {
		doctor := newDoctorService(app, selected.project)
		part, err := doctor.DiagnoseBoards(selected.names, i == 0)
		if err != nil {
			return nil, nil, err
		}
		for j := range part.Boards {
			part.Boards[j].ProjectPath = selected.project.ProjectRoot
		}
		for j := range part.Issues {
			issue := &part.Issues[j]
			if issue.Board != "" {
				issue.ProjectPath = selected.project.ProjectRoot
				if selected.project.OtherProject {
					issue.Fixable = false
					if issue.FixAction != "" {
						issue.FixAction = "In " + selected.project.ProjectRoot + ": " + issue.FixAction
					}
				}
			}
		}
		if fix && !selected.project.OtherProject {
			part, err = doctor.Fix(part)
			if err != nil {
				return nil, nil, err
			}
		}
		report.Boards = append(report.Boards, part.Boards...)
		report.Issues = append(report.Issues, part.Issues...)
		report.Summary.Fixed += part.Summary.Fixed
		report.Summary.FixFailed += part.Summary.FixFailed
	}
	if len(projects) == 0 {
		part, err := newDoctorService(app, app).DiagnoseBoards(nil, true)
		if err != nil {
			return nil, nil, err
		}
		report.Issues = append(report.Issues, part.Issues...)
	}
	if app.globalCfg != nil {
		duplicates := map[string]map[string]bool{}
		for _, status := range registry.Inspect(app.globalCfg, false) {
			for id, paths := range status.Duplicates {
				if duplicates[id] == nil {
					duplicates[id] = map[string]bool{}
				}
				duplicates[id][status.Path] = true
				for _, path := range paths {
					duplicates[id][path] = true
				}
			}
		}
		var ids []string
		for id := range duplicates {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			var paths []string
			for path := range duplicates[id] {
				paths = append(paths, path)
			}
			sort.Strings(paths)
			report.Issues = append(report.Issues, service.Issue{
				Severity: service.SeverityWarning, Code: service.CodeDuplicateBoardID,
				Message:   fmt.Sprintf("Board ID %s is held by multiple visible projects: %s", id, strings.Join(paths, ", ")),
				FixAction: "Choose a visible copy with 'kan registry hide' or repair the board IDs manually",
			})
		}
	}
	for _, issue := range report.Issues {
		if issue.Severity == service.SeverityError {
			report.Summary.Errors++
		} else {
			report.Summary.Warnings++
		}
	}
	sortSkipped(skipped)
	return report, skipped, nil
}
