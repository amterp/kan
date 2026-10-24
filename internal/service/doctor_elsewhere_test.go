package service

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/model"
)

func TestDoctorElsewhere_CyclesUseProjectIdentity(t *testing.T) {
	web := newLinkProject(t, "web", "b_web")
	api := newLinkProject(t, "api", "b_api")
	parent := web.add(t, "Parent")
	source := web.add(t, "Source")
	source.Parent = parent.ID
	if err := api.cards.Create("main", &model.Card{ID: parent.ID, Title: "Foreign", Column: "open", BlockedBy: []string{"b_web:" + source.ID}}); err != nil {
		t.Fatal(err)
	}
	web.setLinks(t, source, "b_api:"+parent.ID)
	doctor := NewDoctorService(web.paths, web.cards, web.boards)
	doctor.SetElsewhere(elsewhereFor(web, registryOf(web, api)), nil)
	report, err := doctor.Diagnose("main")
	if err != nil {
		t.Fatal(err)
	}
	if len(issuesWithCode(report, CodeBlockedByAncestor)) != 0 || len(issuesWithCode(report, CodeUnresolvedLink)) != 0 {
		t.Fatalf("mixed project identities: %+v", report.Issues)
	}
	cycles := issuesWithCode(report, CodeBlockedByCycle)
	if len(cycles) != 1 || !strings.Contains(cycles[0].Message, api.root) || !strings.Contains(cycles[0].Message, "b_api:") || cycles[0].Board != "main" {
		t.Fatalf("cross-project cycle missing: %+v", report.Issues)
	}
}

func TestDoctorElsewhere_FixRechecksRegistryAndExternalFiles(t *testing.T) {
	for _, change := range []string{"revived", "hidden", "duplicate", "future", "unreadable"} {
		t.Run(change, func(t *testing.T) {
			web := newLinkProject(t, "web", "b_web")
			api := newLinkProject(t, "api", "b_api")
			cfg := registryOf(web, api)
			source := web.add(t, "Source")
			web.setLinks(t, source, "b_api:missing")
			doctor := NewDoctorService(web.paths, web.cards, web.boards)
			doctor.SetElsewhere(elsewhereFor(web, cfg), func() (*Elsewhere, error) { return elsewhereFor(web, cfg), nil })
			report, err := doctor.Diagnose("main")
			if err != nil {
				t.Fatal(err)
			}
			absent := issuesWithCode(report, CodeAbsentLink)
			if len(absent) != 1 {
				t.Fatalf("missing initial absent diagnosis: %+v", report.Issues)
			}
			switch change {
			case "revived":
				if err := api.cards.Create("main", &model.Card{ID: "missing", Title: "Revived", Column: "done"}); err != nil {
					t.Fatal(err)
				}
			case "hidden":
				cfg.SetRepoConfig(api.root, model.RepoConfig{Hidden: true})
			case "duplicate":
				clone := newLinkProject(t, "clone", "b_api")
				cfg.SetRepoConfig(clone.root, model.RepoConfig{})
			case "future":
				if err := os.WriteFile(api.paths.BoardConfigPath("main"), []byte("kan_schema = \"board/999\"\nid = \"b_api\"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.WriteFile(api.paths.CardPath("main", "broken"), []byte("invalid JSON"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			before := readCardBytes(t, web.paths, source.ID)
			fixed, err := doctor.Fix(&DiagnosticReport{Issues: absent})
			if err != nil || fixed.Summary.Fixed != 0 || fixed.Summary.FixFailed != 1 {
				t.Fatalf("unsafe fix was not refused: %+v, %v", fixed, err)
			}
			if !bytes.Equal(before, readCardBytes(t, web.paths, source.ID)) {
				t.Fatal("stale absence removed a link")
			}
		})
	}
}
