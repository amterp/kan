package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
)

type doctorLinksFixture struct {
	t      *testing.T
	svc    *CardService
	paths  *config.Paths
	doctor *DoctorService
	source *model.Card
}

func setupDoctorLinks(t *testing.T) *doctorLinksFixture {
	t.Helper()
	svc, paths := setupCardIntegrityTest(t)
	cfg, err := svc.boardStore.Get("main")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ID = "board-main"
	cfg.Columns[0].Category, cfg.Columns[1].Category = model.CategoryOpen, model.CategoryDone
	if err := svc.boardStore.Update(cfg); err != nil {
		t.Fatal(err)
	}
	return &doctorLinksFixture{t: t, svc: svc, paths: paths,
		doctor: NewDoctorService(paths, svc.cardStore, svc.boardStore),
		source: addIntegrityCard(t, svc, "Source", "")}
}

func (f *doctorLinksFixture) save() {
	f.t.Helper()
	if err := f.svc.cardStore.Update("main", f.source); err != nil {
		f.t.Fatal(err)
	}
}

func (f *doctorLinksFixture) write(path, data string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *doctorLinksFixture) board(name, id string) {
	f.t.Helper()
	if err := f.svc.boardStore.Create(&model.BoardConfig{Name: name, ID: id,
		Columns: []model.Column{{Name: "backlog", Category: model.CategoryOpen}}}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *doctorLinksFixture) diagnose() *DiagnosticReport {
	f.t.Helper()
	report, err := f.doctor.Diagnose("main")
	if err != nil {
		f.t.Fatal(err)
	}
	return report
}

func issuesWithCode(report *DiagnosticReport, code string) []Issue {
	var found []Issue
	for _, issue := range report.Issues {
		if issue.Code == code {
			found = append(found, issue)
		}
	}
	return found
}

func TestDoctorLinks_LegacyCustomLinksDoNotHideParents(t *testing.T) {
	for _, value := range []string{`"waiting"`, `["custom", "values"]`, `{"custom":true}`} {
		t.Run(value, func(t *testing.T) {
			f := setupDoctorLinks(t)
			parent := addIntegrityCard(t, f.svc, "Parent", "")
			f.source.Parent = parent.Alias
			f.save()
			before := map[string][]byte{}
			for _, id := range []string{parent.ID, f.source.ID} {
				var raw map[string]json.RawMessage
				if err := json.Unmarshal(readCardBytes(t, f.paths, id), &raw); err != nil {
					t.Fatal(err)
				}
				raw["_v"], raw["blocked_by"], raw["related"] = json.RawMessage(`3`), json.RawMessage(value), json.RawMessage(value)
				data, err := json.Marshal(raw)
				if err != nil {
					t.Fatal(err)
				}
				f.write(f.paths.CardPath("main", id), string(data))
				before[id] = data
			}
			report := f.diagnose()
			if len(issuesWithCode(report, CodeMalformedCard)) != 0 || len(issuesWithCode(report, CodeInvalidParentRef)) != 0 || len(issuesWithCode(report, CodeSchemaOutdated)) != 2 {
				t.Fatalf("legacy custom values produced false diagnostics: %+v", report.Issues)
			}
			for id, data := range before {
				if !bytes.Equal(data, readCardBytes(t, f.paths, id)) {
					t.Fatal("read-only diagnosis changed legacy card")
				}
			}
			var raw map[string]json.RawMessage
			_ = json.Unmarshal(before[f.source.ID], &raw)
			raw["parent"] = json.RawMessage(`"genuinely-missing"`)
			data, _ := json.Marshal(raw)
			f.write(f.paths.CardPath("main", f.source.ID), string(data))
			if len(issuesWithCode(f.diagnose(), CodeInvalidParentRef)) != 1 {
				t.Fatal("tolerant decoding hid a genuine missing parent")
			}
		})
	}
}

func TestDoctorLinks_FixUsesStoreFormatting(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprint(custom), func(t *testing.T) {
			f := setupDoctorLinks(t)
			f.source.BlockedBy = []string{"missing"}
			if custom {
				f.source.CustomFields = map[string]any{"labels": []string{"keep"}, "priority": "high"}
			}
			f.save()
			report := f.diagnose()
			fixed, err := f.doctor.Fix(&DiagnosticReport{Issues: issuesWithCode(report, CodeAbsentLink)})
			if err != nil || fixed.Summary.Fixed != 1 {
				t.Fatalf("repair failed: %+v, %v", fixed, err)
			}
			actual := readCardBytes(t, f.paths, f.source.ID)
			f.source.BlockedBy = nil
			f.save()
			if expected := readCardBytes(t, f.paths, f.source.ID); !bytes.Equal(expected, actual) {
				t.Fatalf("repair did not use store formatting:\n%s\nexpected:\n%s", actual, expected)
			}
		})
	}
}

func TestDoctorLinks_States(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state LinkState
		setup func(*doctorLinksFixture) string
	}{
		{"found", LinkFound, func(f *doctorLinksFixture) string { return addIntegrityCard(f.t, f.svc, "Target", "").ID }},
		{"absent", LinkAbsent, func(f *doctorLinksFixture) string { return "missing" }},
		{"board not here", LinkBoardNotHere, func(f *doctorLinksFixture) string { return "remote:missing" }},
		{"ambiguous board", LinkAmbiguousBoard, func(f *doctorLinksFixture) string {
			f.board("duplicate", "board-main")
			return "board-main:missing"
		}},
		{"future card", LinkUnsupportedSchema, func(f *doctorLinksFixture) string {
			f.write(f.paths.CardPath("main", "future"), `{"_v":999,"blocked_by":42}`)
			return "future"
		}},
		{"future board", LinkUnsupportedSchema, func(f *doctorLinksFixture) string {
			f.board("future", "board-future")
			f.write(f.paths.BoardConfigPath("future"), "kan_schema = \"board/999\"\nid = \"board-future\"\n")
			return "board-future:missing"
		}},
		{"unreadable card", LinkUnreadable, func(f *doctorLinksFixture) string {
			f.write(f.paths.CardPath("main", "broken"), "invalid JSON")
			return "broken"
		}},
		{"incomplete listing", LinkUnreadable, func(f *doctorLinksFixture) string {
			f.write(f.paths.CardPath("main", "broken"), "invalid JSON")
			return "missing"
		}},
		{"unreadable board", LinkUnreadable, func(f *doctorLinksFixture) string {
			f.board("broken", "board-broken")
			f.write(f.paths.BoardConfigPath("broken"), "invalid = [")
			return "board-broken:missing"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupDoctorLinks(t)
			ref := tc.setup(f)
			f.source.BlockedBy, f.source.Related = []string{ref}, []string{ref}
			f.save()
			before := readCardBytes(t, f.paths, f.source.ID)
			report := f.diagnose()
			var links []Issue
			for _, issue := range report.Issues {
				if issue.Code == CodeAbsentLink || issue.Code == CodeUnresolvedLink {
					links = append(links, issue)
					if !strings.Contains(issue.Message, string(tc.state)) || issue.Fixable != (tc.state == LinkAbsent) {
						t.Fatalf("incorrect state or fixability: %+v", issue)
					}
				}
			}
			want := 2
			if tc.state == LinkFound {
				want = 0
			}
			if len(links) != want {
				t.Fatalf("got link issues %+v, expected %d", links, want)
			}
			if !bytes.Equal(before, readCardBytes(t, f.paths, f.source.ID)) {
				t.Fatal("diagnosis wrote the source")
			}
			if tc.state != LinkAbsent {
				if _, err := f.doctor.Fix(&DiagnosticReport{Issues: links}); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, readCardBytes(t, f.paths, f.source.ID)) {
					t.Fatal("fixed a link without confirmed absence")
				}
			}
		})
	}
}

func TestDoctorLinks_FixPreservesOtherDataAndUsesSourceFilename(t *testing.T) {
	f := setupDoctorLinks(t)
	target := addIntegrityCard(t, f.svc, "Target", "")
	f.board("targets", "board-targets")
	f.source.BlockedBy = []string{"board-targets:gone", "board-targets:gone", target.ID}
	f.source.Related = []string{"board-targets:missing", target.ID}
	f.source.Parent = target.Alias
	f.save()
	path := f.paths.CardPath("main", f.source.ID)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(readCardBytes(t, f.paths, f.source.ID), &raw); err != nil {
		t.Fatal(err)
	}
	raw["id"], _ = json.Marshal(target.ID)
	raw["large_number"] = json.RawMessage("90071992547409931234")
	raw["created_at_millis"] = json.RawMessage("9007199254740993")
	raw["updated_at_millis"] = json.RawMessage("9007199254740995")
	raw["comments"] = json.RawMessage(`[{"id":"comment","body":"Keep","author":"tester","created_at_millis":9007199254740993,"updated_at_millis":9007199254740995}]`)
	data, _ := json.Marshal(raw)
	f.write(path, string(data))
	targetBefore := readCardBytes(t, f.paths, target.ID)
	report := f.diagnose()
	absent := issuesWithCode(report, CodeAbsentLink)
	if len(absent) != 2 {
		t.Fatalf("expected one issue per field/ref: %+v", absent)
	}
	fixed, err := f.doctor.Fix(&DiagnosticReport{Issues: absent})
	if err != nil || fixed.Summary.Fixed != 2 {
		t.Fatalf("fix failed: %+v, %v", fixed, err)
	}
	var after map[string]json.RawMessage
	if err := json.Unmarshal(readCardBytes(t, f.paths, f.source.ID), &after); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"blocked_by", "related"} {
		var refs []string
		if err := json.Unmarshal(after[field], &refs); err != nil || !reflect.DeepEqual(refs, []string{target.ID}) {
			t.Fatalf("wrong remaining %s: %s (%v)", field, after[field], err)
		}
		delete(raw, field)
		delete(after, field)
	}
	for key, value := range raw {
		var expected, actual bytes.Buffer
		if err := json.Compact(&expected, value); err != nil {
			t.Fatal(err)
		}
		if err := json.Compact(&actual, after[key]); err != nil || !bytes.Equal(expected.Bytes(), actual.Bytes()) {
			t.Fatalf("fix changed %s: %s -> %s", key, value, after[key])
		}
	}
	if !bytes.Equal(targetBefore, readCardBytes(t, f.paths, target.ID)) {
		t.Fatal("repair changed the card named by a damaged stored ID")
	}
}

func TestDoctorLinks_FixRechecksDisk(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*doctorLinksFixture)
	}{
		{"target revived", func(f *doctorLinksFixture) {
			if err := f.svc.cardStore.Create("main", &model.Card{ID: "missing", Column: "done", Position: "a0"}); err != nil {
				f.t.Fatal(err)
			}
		}},
		{"card unreadable", func(f *doctorLinksFixture) { f.write(f.paths.CardPath("main", "broken"), "invalid") }},
		{"future target", func(f *doctorLinksFixture) { f.write(f.paths.CardPath("main", "missing"), `{"_v":999}`) }},
		{"board ID duplicated", func(f *doctorLinksFixture) { f.board("duplicate", "board-main") }},
		{"board becomes future behind overlay", func(f *doctorLinksFixture) {
			f.write(f.paths.BoardConfigPath("main"), "kan_schema = \"board/999\"\nid = \"board-main\"\n")
		}},
		{"board disappears", func(f *doctorLinksFixture) {
			if err := os.Rename(f.paths.BoardConfigPath("main"), filepath.Join(f.t.TempDir(), "config.toml")); err != nil {
				f.t.Fatal(err)
			}
		}},
		{"future board hides identity", func(f *doctorLinksFixture) {
			f.board("future", "future")
			f.write(f.paths.BoardConfigPath("future"), "kan_schema = \"board/999\"\n")
		}},
		{"source becomes future", func(f *doctorLinksFixture) {
			data := strings.Replace(string(readCardBytes(f.t, f.paths, f.source.ID)), `"_v": 4`, `"_v": 999`, 1)
			f.write(f.paths.CardPath("main", f.source.ID), data)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupDoctorLinks(t)
			f.source.BlockedBy = []string{"board-main:missing"}
			f.save()
			overlay := store.NewOverlay()
			overlay.Put(f.paths.BoardConfigPath("main"), mustReadDoctorFile(t, f.paths.BoardConfigPath("main")))
			f.doctor = NewDoctorService(f.paths, store.NewReadOnlyCardStore(f.paths, overlay), store.NewReadOnlyBoardStore(f.paths, overlay))
			absent := issuesWithCode(f.diagnose(), CodeAbsentLink)
			if len(absent) != 1 {
				t.Fatalf("missing initial absent issue: %+v", absent)
			}
			tc.change(f)
			before := readCardBytes(t, f.paths, f.source.ID)
			fixed, err := f.doctor.Fix(&DiagnosticReport{Issues: absent})
			if err != nil || fixed.Summary.FixFailed != 1 || fixed.Summary.Fixed != 0 {
				t.Fatalf("unsafe fix was not refused: %+v, %v", fixed, err)
			}
			if !bytes.Equal(before, readCardBytes(t, f.paths, f.source.ID)) {
				t.Fatal("unsafe cleanup changed the source")
			}
		})
	}
}

func TestDoctorLinks_DamagedTargetIdentityIsNotAbsent(t *testing.T) {
	f := setupDoctorLinks(t)
	target := addIntegrityCard(t, f.svc, "Target", "")
	f.source.BlockedBy = []string{target.ID}
	f.save()
	targetPath := f.paths.CardPath("main", target.ID)
	data := strings.Replace(string(readCardBytes(t, f.paths, target.ID)), `"id": "`+target.ID+`"`, `"id": "damaged"`, 1)
	f.write(targetPath, data)
	before := readCardBytes(t, f.paths, f.source.ID)
	report := f.diagnose()
	issues := issuesWithCode(report, CodeUnresolvedLink)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, string(LinkUnreadable)) || issues[0].Fixable {
		t.Fatalf("damaged target was treated as absent: %+v", report.Issues)
	}
	if _, err := f.doctor.Fix(report); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, readCardBytes(t, f.paths, f.source.ID)) {
		t.Fatal("removed a link to an existing file with a damaged ID")
	}
}

func mustReadDoctorFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDoctorLinks_FixReloadsSourceAndIsIdempotent(t *testing.T) {
	f := setupDoctorLinks(t)
	f.source.Related = []string{"gone"}
	f.save()
	report := f.diagnose()
	f.source.Title = "Edited after diagnosis"
	f.source.CustomFields = map[string]any{"new_field": "keep this"}
	f.save()
	fixed, err := f.doctor.Fix(report)
	if err != nil || fixed.Summary.Fixed != 1 {
		t.Fatalf("fix failed: %+v, %v", fixed, err)
	}
	stored, err := f.svc.cardStore.Get("main", f.source.ID)
	if err != nil || stored.Title != f.source.Title || stored.CustomFields["new_field"] != "keep this" || len(stored.Related) != 0 {
		t.Fatalf("fix lost newer source data: %+v, %v", stored, err)
	}
	before := readCardBytes(t, f.paths, f.source.ID)
	if _, err := f.doctor.Fix(report); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, readCardBytes(t, f.paths, f.source.ID)) {
		t.Fatal("repeated fix rewrote the card")
	}
}

func TestDoctorLinks_NamesUncategorizedSourceAndTargetColumns(t *testing.T) {
	f := setupDoctorLinks(t)
	target := addIntegrityCard(t, f.svc, "Target", "")
	target.Column = "done"
	if err := f.svc.cardStore.Update("main", target); err != nil {
		t.Fatal(err)
	}
	f.source.BlockedBy = []string{target.ID}
	f.save()
	cfg, _ := f.svc.boardStore.Get("main")
	cfg.Columns[0].Category, cfg.Columns[1].Category = "", "invalid"
	if err := f.svc.boardStore.Update(cfg); err != nil {
		t.Fatal(err)
	}
	issues := issuesWithCode(f.diagnose(), CodeUncategorizedLink)
	if len(issues) != 2 || !strings.Contains(issues[0].Message, "backlog") || !strings.Contains(issues[1].Message, "done") {
		t.Fatalf("category issues must name both columns: %+v", issues)
	}
	for _, issue := range issues {
		if issue.Fixable {
			t.Fatalf("category issue must require manual repair: %+v", issue)
		}
	}
}

func TestDoctorLinks_CyclesAncestorsCategoriesAndReady(t *testing.T) {
	f := setupDoctorLinks(t)
	parent := addIntegrityCard(t, f.svc, "Parent", "")
	child := addIntegrityCard(t, f.svc, "Child", parent.ID)
	other := addIntegrityCard(t, f.svc, "Other", "")
	f.source.BlockedBy = []string{child.ID}
	f.save()
	child.BlockedBy, child.Parent = []string{f.source.ID, parent.ID}, parent.Alias
	if err := f.svc.cardStore.Update("main", child); err != nil {
		t.Fatal(err)
	}
	other.BlockedBy = []string{other.ID}
	if err := f.svc.cardStore.Update("main", other); err != nil {
		t.Fatal(err)
	}
	cfg, _ := f.svc.boardStore.Get("main")
	cfg.Columns[0].Category = ""
	cfg.Ready = model.ReadyConfig{Order: []string{"missing", "backlog", "backlog"}, Exclude: []string{"invalid"}}
	if err := f.svc.boardStore.Update(cfg); err != nil {
		t.Fatal(err)
	}
	report := f.diagnose()
	for code, want := range map[string]int{CodeBlockedByCycle: 2, CodeBlockedByAncestor: 1, CodeInvalidReadyConfig: len(model.ValidateReady(cfg))} {
		if issues := issuesWithCode(report, code); len(issues) != want {
			t.Fatalf("expected %d %s issues, got %+v", want, code, issues)
		}
	}
	if len(issuesWithCode(report, CodeUncategorizedLink)) == 0 {
		t.Fatal("missing uncategorized link diagnostics")
	}
	before := readCardBytes(t, f.paths, child.ID)
	if fixed, err := f.doctor.Fix(report); err != nil || fixed.Summary.Fixed != 0 {
		t.Fatalf("manual issues were fixed: %+v, %v", fixed, err)
	}
	if !bytes.Equal(before, readCardBytes(t, f.paths, child.ID)) {
		t.Fatal("manual repair changed card bytes")
	}
}

func TestDoctorLinks_BoardFilterRetainsOverlappingCrossBoardCycles(t *testing.T) {
	f := setupDoctorLinks(t)
	f.board("alpha", "alpha-id")
	f.board("beta", "beta-id")
	for _, entry := range []struct {
		board string
		card  model.Card
	}{
		{"alpha", model.Card{ID: "a", Column: "backlog", Position: "a0", BlockedBy: []string{"b", "beta-id:c"}}},
		{"alpha", model.Card{ID: "b", Column: "backlog", Position: "a1", BlockedBy: []string{"a"}}},
		{"beta", model.Card{ID: "0-entry", Column: "backlog", Position: "a0", BlockedBy: []string{"alpha-id:a"}}},
		{"beta", model.Card{ID: "c", Column: "backlog", Position: "a1", BlockedBy: []string{"alpha-id:b"}}},
	} {
		if err := f.svc.cardStore.Create(entry.board, &entry.card); err != nil {
			t.Fatal(err)
		}
	}
	for _, board := range []string{"alpha", "beta", ""} {
		report, err := f.doctor.Diagnose(board)
		if err != nil {
			t.Fatal(err)
		}
		cycles := issuesWithCode(report, CodeBlockedByCycle)
		if len(cycles) != 1 || (board != "" && cycles[0].Board != board) {
			t.Fatalf("board %q lost its cyclic component: %+v", board, cycles)
		}
		if board == "beta" && (cycles[0].CardID != "c" || !strings.Contains(cycles[0].Message, "beta:c")) {
			t.Fatalf("cycle path must include a card on the selected board: %+v", cycles)
		}
	}
}

// kan ready ignores related links, so their columns' categories don't matter.
func TestDoctorLinks_RelatedLinksIgnoreCategories(t *testing.T) {
	f := setupDoctorLinks(t)
	target := addIntegrityCard(t, f.svc, "Target", "")
	f.source.Related = []string{target.ID}
	f.save()
	cfg, _ := f.svc.boardStore.Get("main")
	for i := range cfg.Columns {
		cfg.Columns[i].Category = ""
	}
	if err := f.svc.boardStore.Update(cfg); err != nil {
		t.Fatal(err)
	}
	if issues := issuesWithCode(f.diagnose(), CodeUncategorizedLink); len(issues) != 0 {
		t.Fatalf("related links warned about categories: %+v", issues)
	}
}
