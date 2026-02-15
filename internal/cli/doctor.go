package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/amterp/kan/internal/service"
	"github.com/amterp/ra"
)

func registerDoctor(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("doctor")
	cmd.SetDescription("Check board data for consistency issues. Exit 0 if healthy, 1 if errors found.")

	ctx.DoctorFix, _ = ra.NewBool("fix").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Apply automatic fixes for issues with deterministic solutions").
		Register(cmd)

	ctx.DoctorDryRun, _ = ra.NewBool("dry-run").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Show what fixes would be applied without making changes").
		Register(cmd)

	ctx.DoctorBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Check only a specific board (default: all)").
		SetCompletionFunc(completeBoards).
		Register(cmd)

	ctx.DoctorUsed, _ = parent.RegisterCmd(cmd)
}

func runDoctor(boardName string, fix bool, dryRun bool, jsonOutput bool) {
	app, err := NewApp(false)
	if err != nil {
		Fatal(err)
	}

	if err := app.RequireKan(); err != nil {
		Fatal(err)
	}

	// --fix and --dry-run are mutually exclusive
	if fix && dryRun {
		Fatal(fmt.Errorf("--fix and --dry-run cannot be used together"))
	}

	// Validate board exists if specified
	if boardName != "" && !app.BoardStore.Exists(boardName) {
		boards, _ := app.BoardStore.List()
		if len(boards) == 0 {
			Fatal(fmt.Errorf("board %q not found (no boards exist)", boardName))
		} else {
			Fatal(fmt.Errorf("board %q not found (available: %s)", boardName, strings.Join(boards, ", ")))
		}
	}

	doctorService := service.NewDoctorService(app.Paths)

	// Run diagnosis
	report, err := doctorService.Diagnose(boardName)
	if err != nil {
		Fatal(err)
	}

	// Apply fixes if requested (not in dry-run mode)
	if fix && len(report.Issues) > 0 {
		report, err = doctorService.Fix(report)
		if err != nil {
			Fatal(err)
		}
	}

	if jsonOutput {
		if err := printJson(report); err != nil {
			Fatal(err)
		}
	} else {
		printDoctorReport(report, fix, dryRun)
	}

	// Exit with status 1 if there are errors
	if report.HasErrors() {
		os.Exit(1)
	}
}

func printDoctorReport(report *service.DiagnosticReport, didFix bool, dryRun bool) {
	// Print board stats
	for _, board := range report.Boards {
		fmt.Printf("Checking board %s...\n", RenderBold(fmt.Sprintf("%q", board.Name)))
		fmt.Printf("  Cards: %d files, %d referenced\n", board.CardFiles, board.CardsReferenced)
		fmt.Printf("  Columns: %d\n", board.Columns)
		fmt.Println()
	}

	if len(report.Boards) == 0 {
		PrintInfo("No boards found")
		return
	}

	// Print issues
	fixedCount := 0
	if didFix {
		fixedCount = report.Summary.Fixed
	}

	if fixedCount > 0 {
		PrintSuccess("Fixed %d issue(s)", fixedCount)
		fmt.Println()
	}

	// In dry-run mode, show what would be fixed
	if dryRun {
		fixableCount := 0
		for _, issue := range report.Issues {
			if issue.Fixable {
				fixableCount++
			}
		}
		if fixableCount > 0 {
			PrintInfo("Dry run: %d issue(s) would be fixed", fixableCount)
			fmt.Println()
		}
	}

	if len(report.Issues) == 0 {
		if fixedCount == 0 {
			PrintSuccess("No issues found")
		} else {
			PrintSuccess("All issues resolved")
		}
		return
	}

	// Group issues by severity
	var errors, warnings []service.Issue
	for _, issue := range report.Issues {
		if issue.Severity == service.SeverityError {
			errors = append(errors, issue)
		} else {
			warnings = append(warnings, issue)
		}
	}

	// Print errors first
	for _, issue := range errors {
		printIssue(issue)
	}

	// Then warnings
	for _, issue := range warnings {
		printIssue(issue)
	}

	// Summary
	fmt.Println()
	summaryParts := []string{}
	if report.Summary.Errors > 0 {
		summaryParts = append(summaryParts, StyleError.Render(fmt.Sprintf("%d error(s)", report.Summary.Errors)))
	}
	if report.Summary.Warnings > 0 {
		summaryParts = append(summaryParts, StyleWarning.Render(fmt.Sprintf("%d warning(s)", report.Summary.Warnings)))
	}
	if fixedCount > 0 {
		summaryParts = append(summaryParts, StyleSuccess.Render(fmt.Sprintf("%d fixed", fixedCount)))
	}
	if report.Summary.FixFailed > 0 {
		summaryParts = append(summaryParts, StyleError.Render(fmt.Sprintf("%d fix failed", report.Summary.FixFailed)))
	}

	fmt.Printf("Summary: %s\n", joinParts(summaryParts))

	// Suggest --fix if there are fixable issues
	if !didFix {
		hasFixable := false
		for _, issue := range report.Issues {
			if issue.Fixable {
				hasFixable = true
				break
			}
		}
		if hasFixable {
			fmt.Println()
			if dryRun {
				PrintInfo("Run 'kan doctor --fix' to apply these fixes")
			} else {
				PrintInfo("Run 'kan doctor --fix' to apply automatic fixes")
			}
		}
	}
}

func printIssue(issue service.Issue) {
	var icon, code string
	if issue.Severity == service.SeverityError {
		icon = StyleError.Render(IconError)
		code = StyleError.Render(fmt.Sprintf("[%s]", issue.Code))
	} else {
		icon = StyleWarning.Render(IconWarning)
		code = StyleWarning.Render(fmt.Sprintf("[%s]", issue.Code))
	}

	location := ""
	if issue.Board != "" {
		location = fmt.Sprintf(" %s", RenderMuted(issue.Board))
		if issue.CardID != "" {
			location += fmt.Sprintf("/%s", RenderID(issue.CardID))
		}
	}

	fmt.Printf("%s %s%s %s\n", icon, code, location, issue.Message)

	if issue.FixError != "" {
		fmt.Printf("  %s Fix failed: %s\n", StyleError.Render("→"), issue.FixError)
	} else if issue.FixAction != "" {
		if issue.Fixable {
			fmt.Printf("  %s Fix: %s\n", RenderMuted("→"), issue.FixAction)
		} else {
			fmt.Printf("  %s %s\n", RenderMuted("→"), issue.FixAction)
		}
	}
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
