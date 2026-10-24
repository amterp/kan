package service

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

func TestLinkService_StoredIDsAndNoOpWrites(t *testing.T) {
	svc, paths := setupCardIntegrityTest(t)
	a := addIntegrityCard(t, svc, "Source", "")
	b := addIntegrityCard(t, svc, "Target", "")
	links := NewLinkService(svc.cardStore, svc.boardStore)
	updated, err := links.Link("main", a.Alias, []string{b.Alias, b.ID}, []string{b.Alias})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.BlockedBy) != 1 || updated.BlockedBy[0] != b.ID || len(updated.Related) != 1 || updated.Related[0] != b.ID {
		t.Fatalf("links were not canonical and deduplicated: %+v", updated)
	}
	before := readCardBytes(t, paths, a.ID)
	readOnlyLinks := NewLinkService(store.NewReadOnlyCardStore(paths, nil), svc.boardStore)
	if _, err := readOnlyLinks.Link("main", a.ID, []string{b.ID}, []string{b.Alias}); err != nil {
		t.Fatalf("repeating links should not write: %v", err)
	}
	if _, err := readOnlyLinks.Unlink("main", a.ID, []string{"does-not-exist"}); err != nil {
		t.Fatalf("removing absent links should not write: %v", err)
	}
	if !bytes.Equal(before, readCardBytes(t, paths, a.ID)) {
		t.Fatal("no-op changed card bytes")
	}
	if err := svc.UpdateTitle("main", b, "Renamed target"); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.Get("main", a.ID)
	if err != nil || stored.BlockedBy[0] != b.ID {
		t.Fatalf("renaming broke stored links: %+v, %v", stored, err)
	}
}

func TestLinkService_RejectsCyclesAndAncestorBlockers(t *testing.T) {
	svc, paths := setupCardIntegrityTest(t)
	a := addIntegrityCard(t, svc, "A", "")
	b := addIntegrityCard(t, svc, "B", "")
	c := addIntegrityCard(t, svc, "C", "")
	links := NewLinkService(svc.cardStore, svc.boardStore)
	if _, err := links.Link("main", a.ID, []string{b.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := links.Link("main", b.ID, []string{c.ID}, nil); err != nil {
		t.Fatal(err)
	}
	before := readCardBytes(t, paths, c.ID)
	for _, blocker := range []string{a.ID, c.ID} {
		if _, err := links.Link("main", c.ID, []string{blocker}, nil); err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("expected blocker cycle error, got %v", err)
		}
	}
	if !bytes.Equal(before, readCardBytes(t, paths, c.ID)) {
		t.Fatal("rejected cycle changed source")
	}
	if _, err := links.Link("main", c.ID, nil, []string{a.ID}); err != nil {
		t.Fatalf("related links must not act as blockers: %v", err)
	}
	child := addIntegrityCard(t, svc, "Child", a.ID)
	grandchild := addIntegrityCard(t, svc, "Grandchild", child.ID)
	for _, ancestor := range []string{a.ID, child.ID} {
		if _, err := links.Link("main", grandchild.ID, []string{ancestor}, nil); err == nil || !strings.Contains(err.Error(), "ancestor") {
			t.Fatalf("expected ancestor blocker error, got %v", err)
		}
	}
}

func TestLinkService_UnlinkDanglingRefsAndBothKinds(t *testing.T) {
	svc, _ := setupCardIntegrityTest(t)
	a := addIntegrityCard(t, svc, "Source", "")
	b := addIntegrityCard(t, svc, "Target", "")
	a.BlockedBy = []string{b.ID, "deleted", "elsewhere:gone"}
	a.Related = []string{b.ID, "deleted", "keep"}
	a.Parent = "invalid-parent"
	if err := svc.cardStore.Update("main", a); err != nil {
		t.Fatal(err)
	}
	card, err := NewLinkService(svc.cardStore, svc.boardStore).Unlink("main", a.ID, []string{b.Alias, "deleted", "elsewhere:gone"})
	if err != nil {
		t.Fatal(err)
	}
	if len(card.BlockedBy) != 0 || len(card.Related) != 1 || card.Related[0] != "keep" || card.Parent != "invalid-parent" {
		t.Fatalf("unlink changed unrelated fields or retained links: %+v", card)
	}
}

func TestCardService_AddBlockersBeforePersistence(t *testing.T) {
	svc, _ := setupCardIntegrityTest(t)
	parent := addIntegrityCard(t, svc, "Parent", "")
	before, _ := svc.cardStore.List("main")
	for _, tc := range []AddCardInput{
		{BoardName: "main", Title: "Missing", BlockedBy: []string{"missing"}},
		{BoardName: "main", Title: "Unknown board", BlockedBy: []string{"b_nowhere:" + parent.ID}},
		{BoardName: "main", Title: "Ancestor", Parent: parent.ID, BlockedBy: []string{parent.Alias}},
	} {
		if _, _, err := svc.Add(tc); err == nil {
			t.Fatalf("accepted invalid blocker: %+v", tc)
		}
	}
	after, _ := svc.cardStore.List("main")
	if len(after) != len(before) {
		t.Fatalf("rejected blocker left cards: %+v", after)
	}
	created, _, err := svc.Add(AddCardInput{BoardName: "main", Title: "Valid", BlockedBy: []string{parent.Alias}})
	if err != nil || len(created.BlockedBy) != 1 || created.BlockedBy[0] != parent.ID {
		t.Fatalf("add did not store canonical blockers: %+v, %v", created, err)
	}
}

func TestLinkGraph_StatesAndSnapshot(t *testing.T) {
	svc, paths := setupCardIntegrityTest(t)
	cfg, _ := svc.boardStore.Get("main")
	cfg.ID = "board-main"
	cfg.Columns[0].Category = model.CategoryOpen
	if err := svc.boardStore.Update(cfg); err != nil {
		t.Fatal(err)
	}
	target := addIntegrityCard(t, svc, "Target", "")
	waiter := addIntegrityCard(t, svc, "Waiter", "")
	waiter.BlockedBy, waiter.Related = []string{target.ID}, []string{target.ID}
	if err := svc.cardStore.Update("main", waiter); err != nil {
		t.Fatal(err)
	}
	graph, err := LoadLinkGraph(svc.boardStore, svc.cardStore)
	if err != nil {
		t.Fatal(err)
	}
	found := graph.Resolve("main", "board-main:"+target.ID)
	if found.State != LinkFound || found.Card.ID != target.ID || found.Column.Category != model.CategoryOpen {
		t.Fatalf("unexpected found result: %+v", found)
	}
	if result := graph.Resolve("main", "missing"); result.State != LinkAbsent {
		t.Fatalf("expected absent: %+v", result)
	}
	if result := graph.Resolve("main", "remote:missing"); result.State != LinkBoardNotHere {
		t.Fatalf("expected board not here: %+v", result)
	}
	if len(graph.Blocks("main", target.ID)) != 1 || len(graph.RelatedTo("main", target.ID)) != 1 || len(graph.Blockers("main", waiter)) != 1 {
		t.Fatal("forward or reverse links missing")
	}
	if err := svc.Delete("main", target.ID); err != nil {
		t.Fatal(err)
	}
	if graph.Resolve("main", target.ID).State != LinkFound {
		t.Fatal("snapshot reread a card after loading")
	}
	if err := os.WriteFile(paths.CardPath("main", "broken"), []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.CardPath("main", "future"), []byte(`{"_v":999,"id":"future","blocked_by":42}`), 0644); err != nil {
		t.Fatal(err)
	}
	graph, err = LoadLinkGraph(svc.boardStore, svc.cardStore)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"broken", "missing", target.ID} {
		if result := graph.Resolve("main", ref); result.State != LinkUnreadable {
			t.Fatalf("incomplete listing cannot establish absence of %s: %+v", ref, result)
		}
	}
	if result := graph.Resolve("main", "future"); result.State != LinkUnsupportedSchema || result.MinKanVersion == "" {
		t.Fatalf("expected unsupported schema before field decoding: %+v", result)
	}
	if result := graph.Resolve("main", waiter.ID); result.State != LinkFound {
		t.Fatalf("unrelated unreadable files hid a found card: %+v", result)
	}
	if err := svc.boardStore.Create(&model.BoardConfig{Name: "duplicate", ID: "board-main", Columns: []model.Column{{Name: "todo"}}}); err != nil {
		t.Fatal(err)
	}
	graph, _ = LoadLinkGraph(svc.boardStore, svc.cardStore)
	if result := graph.Resolve("main", "board-main:"+waiter.ID); result.State != LinkAmbiguousBoard || !strings.Contains(result.Reason, "duplicate") {
		t.Fatalf("expected ambiguous board: %+v", result)
	}
	if err := os.WriteFile(paths.BoardConfigPath("duplicate"), []byte("kan_schema = \"board/999\"\nid = \"board-future\"\ncolumns = \"new format\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	graph, _ = LoadLinkGraph(svc.boardStore, svc.cardStore)
	if result := graph.Resolve("main", "board-future:card"); result.State != LinkUnsupportedSchema {
		t.Fatalf("expected unsupported board: %+v", result)
	}
}

func TestLinkGraph_QualifiedRefsRequireKnownBoardIdentities(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  string
		symlink bool
		want    LinkState
	}{
		{name: "config stat fails", symlink: true, want: LinkUnreadable},
		{name: "future config without ID", config: "kan_schema = \"board/999\"\n", want: LinkUnsupportedSchema},
		{name: "current config without ID", config: "kan_schema = \"" + version.CurrentBoardSchema() + "\"\n", want: LinkUnreadable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, paths := setupCardIntegrityTest(t)
			cfg, err := svc.boardStore.Get("main")
			if err != nil {
				t.Fatal(err)
			}
			cfg.ID = "board-main"
			if err := svc.boardStore.Update(cfg); err != nil {
				t.Fatal(err)
			}
			target := addIntegrityCard(t, svc, "Target", "")
			if err := os.MkdirAll(paths.BoardDir("unknown"), 0755); err != nil {
				t.Fatal(err)
			}
			if tc.symlink {
				if runtime.GOOS == "windows" {
					t.Skip("symlink creation requires privileges on Windows")
				}
				if err := os.Symlink("config.toml", paths.BoardConfigPath("unknown")); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(paths.BoardConfigPath("unknown"), []byte(tc.config), 0644); err != nil {
				t.Fatal(err)
			}
			graph, err := LoadLinkGraph(svc.boardStore, svc.cardStore)
			if err != nil {
				t.Fatal(err)
			}
			for _, ref := range []string{"remote:missing", "board-main:" + target.ID} {
				result := graph.Resolve("main", ref)
				if result.State != tc.want || result.Reason == "" {
					t.Fatalf("unavailable board identity cannot establish absence or uniqueness of %s: %+v", ref, result)
				}
			}
			if result := graph.Resolve("main", target.ID); result.State != LinkFound {
				t.Fatalf("unknown identity hid a same-board card: %+v", result)
			}
		})
	}
}
