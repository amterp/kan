package service

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

func TestMigrateService_CardV4CollisionsAndParents(t *testing.T) {
	svc, dir, cleanup := setupMigrationTest(t, "card4-collisions")
	defer cleanup()
	paths := config.NewPaths(dir, "")
	before := readCardBytes(t, paths, "card-abc")
	plan, err := svc.PlanProject()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.HasChanges() || !plan.Boards[0].NeedsMigration || plan.Boards[0].FromSchema != "board/14" || len(plan.Boards[0].FieldRenames) != 2 {
		t.Fatalf("card migration must plan board content changes at board/14: %+v", plan)
	}
	var output bytes.Buffer
	svc.output = &output
	if err := svc.Execute(plan, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Would rename custom field blocked_by to x_blocked_by") || !strings.Contains(output.String(), "related to x_related") {
		t.Fatalf("dry-run omitted renames: %s", output.String())
	}
	overlay, err := svc.MigrateInMemory(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, readCardBytes(t, paths, "card-abc")) {
		t.Fatal("read overlay wrote to disk")
	}
	cards := store.NewReadOnlyCardStore(paths, overlay)
	card, err := cards.Get("main", "card-abc")
	if err != nil {
		t.Fatal(err)
	}
	if card.Version != 4 || len(card.BlockedBy) != 0 || len(card.Related) != 0 || card.CustomFields["x_blocked_by"] != "waiting" || card.CustomFields["x_related"] == nil {
		t.Fatalf("legacy custom values were lost or mistaken for links: %+v", card)
	}
	child, err := cards.Get("main", "child")
	if err != nil || child.Parent != card.ID {
		t.Fatalf("legacy parent alias was not rewritten: %+v, %v", child, err)
	}
	board, err := store.NewReadOnlyBoardStore(paths, overlay).Get("main")
	if err != nil {
		t.Fatal(err)
	}
	if board.KanSchema != "board/14" || board.CustomFields["x_related"].Type != "free-set" || board.CardDisplay.DefaultSort != "x_blocked_by" || board.CardDisplay.Badges[1] != "x_related" || board.Ready.Exclude[0] != "x_blocked_by=waiting" || board.Ready.Exclude[1] != "x_related=annotation" {
		t.Fatalf("board references were not renamed consistently: %+v", board)
	}
	if err := svc.Execute(plan, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.BoardConfigPath("main"), paths.CardPath("main", "card-abc"), paths.CardPath("main", "child")} {
		expected, ok := overlay.Get(path)
		actual, err := os.ReadFile(path)
		if !ok || err != nil || !bytes.Equal(expected, actual) {
			t.Fatalf("overlay and disk differ for %s: overlay=%t err=%v", path, ok, err)
		}
	}
	plan, err = svc.PlanProject()
	if err != nil || plan.HasChanges() {
		t.Fatalf("card/4 migration is not idempotent: %+v, %v", plan, err)
	}
}

func TestMigrateService_CardV4ConflictsFailBeforeWrites(t *testing.T) {
	for _, source := range []string{"definition", "card"} {
		t.Run(source, func(t *testing.T) {
			svc, dir, cleanup := setupMigrationTest(t, "card4-collisions")
			defer cleanup()
			paths := config.NewPaths(dir, "")
			if source == "definition" {
				f, err := os.OpenFile(paths.BoardConfigPath("main"), os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.WriteString("\n[custom_fields.x_related]\ntype = \"string\"\n")
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
			} else {
				data := readCardBytes(t, paths, "card-abc")
				var raw map[string]any
				if err := json.Unmarshal(data, &raw); err != nil {
					t.Fatal(err)
				}
				raw["x_related"] = "preserve me"
				data, _ = json.Marshal(raw)
				if err := os.WriteFile(paths.CardPath("main", "card-abc"), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			before := readCardBytes(t, paths, "card-abc")
			if _, err := svc.PlanProject(); err == nil || !strings.Contains(err.Error(), "destination already exists") {
				t.Fatalf("expected rename conflict before writes, got %v", err)
			}
			if !bytes.Equal(before, readCardBytes(t, paths, "card-abc")) {
				t.Fatal("failed plan changed card bytes")
			}
		})
	}
}

func TestMigrateLinksRaw_EmptyMixedAndOrphanedFields(t *testing.T) {
	old := &rawCard{data: map[string]any{"_v": float64(3), "id": "old", "alias": "old-alias", "related": "orphaned custom value", "parent": "new-alias"}}
	current := &rawCard{data: map[string]any{"_v": float64(4), "id": "new", "alias": "new-alias", "related": []any{"old"}}}
	b := &rawBoard{name: "mixed", config: map[string]any{}, cards: []*rawCard{old, current}}
	_, _, err := migrateLinksRaw(b)
	if err != nil || old.data["x_related"] != "orphaned custom value" || old.data["parent"] != "new" || current.data["related"] == nil {
		t.Fatalf("mixed migration lost values: old=%+v current=%+v err=%v", old.data, current.data, err)
	}
	b = &rawBoard{name: "empty", config: map[string]any{"custom_fields": map[string]any{"blocked_by": map[string]any{"type": "string"}}}}
	_, changed, err := migrateLinksRaw(b)
	if err != nil || !changed || b.config["custom_fields"].(map[string]any)["x_blocked_by"] == nil {
		t.Fatalf("empty board collision not migrated: %+v, %v", b, err)
	}
}

func TestMigrateService_CardV3ToV4AndCurrentNoOp(t *testing.T) {
	for _, fixture := range []string{"v14", "v15"} {
		t.Run(fixture, func(t *testing.T) {
			svc, dir, cleanup := setupMigrationTest(t, fixture)
			defer cleanup()
			plan, err := svc.PlanProject()
			if err != nil {
				t.Fatal(err)
			}
			if plan.HasChanges() != (fixture == "v14") {
				t.Fatalf("unexpected migration requirement for %s", fixture)
			}
			if err := svc.Execute(plan, false); err != nil {
				t.Fatal(err)
			}
			card, err := store.NewCardStore(config.NewPaths(dir, "")).Get("main", "card-abc")
			if err != nil || card.Version != version.CurrentCardVersion {
				t.Fatalf("migrated card unreadable: %+v, %v", card, err)
			}
		})
	}
}
