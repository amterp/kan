package service

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/model"
)

func TestDoctorService_ParentGraphs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		parents  map[string]string
		wantCode string
		wantIDs  []string
	}{
		{name: "valid ID", parents: map[string]string{"card-1": "card-2"}},
		{name: "valid legacy alias", parents: map[string]string{"card-1": "c2"}},
		{name: "self ID", parents: map[string]string{"card-1": "card-1"}, wantCode: CodeSelfParent, wantIDs: []string{"card-1"}},
		{name: "self alias", parents: map[string]string{"card-1": "c1"}, wantCode: CodeSelfParent, wantIDs: []string{"card-1"}},
		{name: "two cards", parents: map[string]string{"card-1": "card-2", "card-2": "card-1"}, wantCode: CodeParentCycle, wantIDs: []string{"card-1", "card-2"}},
		{name: "legacy alias cycle", parents: map[string]string{"card-1": "c2", "card-2": "c1"}, wantCode: CodeParentCycle, wantIDs: []string{"card-1", "card-2"}},
		{name: "long cycle", parents: map[string]string{"card-1": "card-2", "card-2": "card-3", "card-3": "card-1"}, wantCode: CodeParentCycle, wantIDs: []string{"card-1", "card-2", "card-3"}},
		{name: "descendant of cycle", parents: map[string]string{"card-1": "card-2", "card-2": "card-1", "card-3": "card-1"}, wantCode: CodeParentCycle, wantIDs: []string{"card-1", "card-2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doctor, _, cleanup := setupDoctorTest(t, "healthy")
			defer cleanup()
			if _, needed := tc.parents["card-3"]; needed {
				if err := doctor.cardStore.Create("main", &model.Card{ID: "card-3", Alias: "c3", Title: "Third", Column: "backlog", Position: "a1"}); err != nil {
					t.Fatal(err)
				}
			}
			before := map[string][]byte{}
			for id, parent := range tc.parents {
				card, err := doctor.cardStore.Get("main", id)
				if err != nil {
					t.Fatal(err)
				}
				card.Parent = parent
				if err := doctor.cardStore.Update("main", card); err != nil {
					t.Fatal(err)
				}
				before[id] = readCardBytes(t, doctor.paths, id)
			}
			report, err := doctor.Diagnose("main")
			if err != nil {
				t.Fatal(err)
			}
			wantCount := 0
			if tc.wantCode != "" {
				wantCount = 1
			}
			if len(report.Issues) != wantCount {
				t.Fatalf("expected %d issues, got %+v", wantCount, report.Issues)
			}
			if wantCount > 0 {
				issue := report.Issues[0]
				if issue.Code != tc.wantCode || issue.Fixable || issue.Severity != SeverityWarning || issue.Board != "main" {
					t.Fatalf("unexpected parent issue: %+v", issue)
				}
				for _, id := range tc.wantIDs {
					if !strings.Contains(issue.Message, id) {
						t.Errorf("issue omits %s: %+v", id, issue)
					}
				}
			}
			fixed, err := doctor.Fix(report)
			if err != nil || fixed.Summary.Fixed != 0 || len(fixed.Issues) != wantCount {
				t.Fatalf("parent graph was auto-fixed: %+v, %v", fixed, err)
			}
			for id, data := range before {
				if !bytes.Equal(data, readCardBytes(t, doctor.paths, id)) {
					t.Errorf("doctor changed parent file %s", id)
				}
			}
		})
	}
}

func TestDoctorService_MissingParentFixUsesFilename(t *testing.T) {
	doctor, _, cleanup := setupDoctorTest(t, "healthy")
	defer cleanup()
	if err := doctor.cardStore.Create("main", &model.Card{ID: "card-3", Alias: "c3", Title: "Third", Column: "backlog", Position: "a1"}); err != nil {
		t.Fatal(err)
	}
	other, err := doctor.cardStore.Get("main", "card-2")
	if err != nil {
		t.Fatal(err)
	}
	other.Parent = "card-3"
	if err := doctor.cardStore.Update("main", other); err != nil {
		t.Fatal(err)
	}
	otherBefore := readCardBytes(t, doctor.paths, "card-2")
	card, err := doctor.cardStore.Get("main", "card-1")
	if err != nil {
		t.Fatal(err)
	}
	card.ID = "card-2"
	card.Parent = "missing"
	data, err := card.MarshalFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doctor.paths.CardPath("main", "card-1"), data, 0644); err != nil {
		t.Fatal(err)
	}
	report, err := doctor.Diagnose("main")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Code != CodeInvalidParentRef || report.Issues[0].CardID != "card-1" {
		t.Fatalf("missing-parent issue must name its source filename: %+v", report.Issues)
	}
	fixed, err := doctor.Fix(report)
	if err != nil || fixed.Summary.Fixed != 1 {
		t.Fatalf("missing-parent repair failed: %+v, %v", fixed, err)
	}
	if !bytes.Equal(otherBefore, readCardBytes(t, doctor.paths, "card-2")) {
		t.Fatal("missing-parent fix changed an unrelated card")
	}
	stored, err := doctor.cardStore.Get("main", "card-1")
	if err != nil || stored.Parent != "" {
		t.Fatalf("source file was not repaired: %+v, %v", stored, err)
	}
}
