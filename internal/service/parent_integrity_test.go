package service

import (
	"bytes"
	"strings"
	"testing"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/model"
)

func addIntegrityCard(t *testing.T, svc *CardService, title, parent string) *model.Card {
	t.Helper()
	card, _, err := svc.Add(AddCardInput{BoardName: "main", Title: title, Parent: parent})
	if err != nil {
		t.Fatal(err)
	}
	return card
}

func TestCardService_ParentAliasesStoreIDs(t *testing.T) {
	for _, operation := range []string{"add", "edit", "update", "restore"} {
		t.Run(operation, func(t *testing.T) {
			svc, _ := setupCardIntegrityTest(t)
			parent := addIntegrityCard(t, svc, "Parent", "")
			var child *model.Card
			if operation == "add" {
				child = addIntegrityCard(t, svc, "Child", parent.Alias)
			} else {
				child = addIntegrityCard(t, svc, "Child", "")
				var err error
				switch operation {
				case "edit":
					child, err = svc.Edit(EditCardInput{BoardName: "main", CardIDOrAlias: child.ID, Parent: &parent.Alias})
				case "update":
					child.Parent = parent.Alias
					err = svc.Update("main", child)
				case "restore":
					if err := svc.Delete("main", child.ID); err != nil {
						t.Fatal(err)
					}
					child.Parent = parent.Alias
					err = svc.Restore("main", child, "backlog", -1)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if child.Parent != parent.ID {
				t.Fatalf("returned parent %q, want %q", child.Parent, parent.ID)
			}
			if err := svc.UpdateTitle("main", parent, "Renamed parent"); err != nil {
				t.Fatal(err)
			}
			stored, err := svc.Get("main", child.ID)
			if err != nil || stored.Parent != parent.ID {
				t.Fatalf("parent changed after renaming: %+v, %v", stored, err)
			}
		})
	}
}

func TestCardService_RejectsInvalidParentsBeforeEditing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ref    string
		legacy bool
		want   string
	}{
		{name: "self ID", ref: "self-id", want: "own parent"},
		{name: "self alias", ref: "self-alias", want: "own parent"},
		{name: "two cards", ref: "child", want: "parent cycle"},
		{name: "long cycle", ref: "grandchild", want: "parent cycle"},
		{name: "legacy aliases", ref: "grandchild", legacy: true, want: "parent cycle"},
		{name: "missing", ref: "missing", want: "parent card not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, paths := setupCardIntegrityTest(t)
			root := addIntegrityCard(t, svc, "Root", "")
			child := addIntegrityCard(t, svc, "Child", root.ID)
			grandchild := addIntegrityCard(t, svc, "Grandchild", child.ID)
			if tc.legacy {
				child.Parent = root.Alias
				grandchild.Parent = child.Alias
				for _, card := range []*model.Card{child, grandchild} {
					if err := svc.cardStore.Update("main", card); err != nil {
						t.Fatal(err)
					}
				}
			}
			refs := map[string]string{"self-id": root.ID, "self-alias": root.Alias, "child": child.ID, "grandchild": grandchild.Alias, "missing": "missing"}
			ref := refs[tc.ref]
			before := readCardBytes(t, paths, root.ID)
			title, column := "Changed title", "done"
			_, err := svc.Edit(EditCardInput{BoardName: "main", CardIDOrAlias: root.ID, Parent: &ref, Title: &title, Column: &column, Alias: &root.Alias})
			if err == nil || !strings.Contains(err.Error(), tc.want) || !kanerr.IsValidationError(err) {
				t.Fatalf("expected %q validation error, got %v", tc.want, err)
			}
			if tc.ref != "missing" && !strings.Contains(err.Error(), root.ID) {
				t.Errorf("error does not identify the card: %v", err)
			}
			if !bytes.Equal(before, readCardBytes(t, paths, root.ID)) {
				t.Fatal("rejected parent edit changed the card file")
			}
		})
	}
}

func TestCardService_UpdateRejectsParentCycle(t *testing.T) {
	svc, paths := setupCardIntegrityTest(t)
	root := addIntegrityCard(t, svc, "Root", "")
	child := addIntegrityCard(t, svc, "Child", root.ID)
	before := readCardBytes(t, paths, root.ID)
	root.Parent = child.Alias
	if err := svc.Update("main", root); err == nil || !strings.Contains(err.Error(), child.ID) {
		t.Fatalf("expected a cycle identifying child %s, got %v", child.ID, err)
	}
	if !bytes.Equal(before, readCardBytes(t, paths, root.ID)) {
		t.Fatal("rejected update changed the card")
	}
}

func TestCardService_NewCardRejectsParentCycles(t *testing.T) {
	for _, operation := range []string{"add", "restore"} {
		t.Run(operation, func(t *testing.T) {
			svc, _ := setupCardIntegrityTest(t)
			parent := addIntegrityCard(t, svc, "Parent", "")
			parent.Parent = "new-card"
			if err := svc.cardStore.Update("main", parent); err != nil {
				t.Fatal(err)
			}
			var err error
			if operation == "add" {
				svc.generateCardID = func() string { return "new-card" }
				_, _, err = svc.Add(AddCardInput{BoardName: "main", Title: "New", Parent: parent.Alias})
			} else {
				err = svc.Restore("main", &model.Card{ID: "new-card", Title: "Restored", Parent: parent.Alias}, "backlog", -1)
			}
			if err == nil || !strings.Contains(err.Error(), "parent cycle") {
				t.Fatalf("expected a cycle, got %v", err)
			}
			if _, err := svc.Get("main", "new-card"); !kanerr.IsNotFound(err) {
				t.Fatalf("rejected creation left a card: %v", err)
			}
		})
	}
}

func TestCardService_ParentsStayWithinBoard(t *testing.T) {
	svc, _ := setupCardIntegrityTest(t)
	if err := svc.boardStore.Create(&model.BoardConfig{Name: "other", DefaultColumn: "backlog", Columns: []model.Column{{Name: "backlog"}}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.cardStore.Create("other", &model.Card{ID: "other-parent", Alias: "external", Title: "External"}); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"other-parent", "external", "other:other-parent"} {
		if _, _, err := svc.Add(AddCardInput{BoardName: "main", Title: "Child", Parent: ref}); err == nil {
			t.Errorf("accepted parent %q from another board", ref)
		}
	}
}

func TestCardService_ClearInvalidParent(t *testing.T) {
	svc, _ := setupCardIntegrityTest(t)
	card := addIntegrityCard(t, svc, "Original", "")
	card.Parent = card.Alias
	if err := svc.cardStore.Update("main", card); err != nil {
		t.Fatal(err)
	}
	empty, title, column := "", "Fixed", "done"
	updated, err := svc.Edit(EditCardInput{BoardName: "main", CardIDOrAlias: card.ID, Parent: &empty, Title: &title, Column: &column})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := svc.Get("main", card.ID)
	if err != nil || stored.Parent != "" || stored.Title != title || stored.Column != column || updated.Parent != "" {
		t.Fatalf("clearing invalid parent failed: %+v, %v", stored, err)
	}
}

func TestCardService_AddRefusesExistingParentCycle(t *testing.T) {
	svc, _ := setupCardIntegrityTest(t)
	parent := addIntegrityCard(t, svc, "Parent", "")
	parent.Parent = parent.Alias
	if err := svc.cardStore.Update("main", parent); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Add(AddCardInput{BoardName: "main", Title: "Child", Parent: parent.ID}); err == nil || !strings.Contains(err.Error(), parent.ID) {
		t.Fatalf("expected existing cycle to be refused, got %v", err)
	}
}

func TestCardService_ParentIDWinsOverAlias(t *testing.T) {
	svc, _ := setupCardIntegrityTest(t)
	parent := addIntegrityCard(t, svc, "Parent", "")
	other := addIntegrityCard(t, svc, "Other", "")
	other.Alias = parent.ID
	if err := svc.cardStore.Update("main", other); err != nil {
		t.Fatal(err)
	}
	child := addIntegrityCard(t, svc, "Child", parent.ID)
	if child.Parent != parent.ID {
		t.Fatalf("stored parent %q, want exact ID %q", child.Parent, parent.ID)
	}
}

func TestCardService_FinalAliasesCannotCloseParentCycles(t *testing.T) {
	for _, operation := range []string{"add", "restore", "restore regenerated alias", "edit title", "edit alias", "update"} {
		t.Run(operation, func(t *testing.T) {
			svc, paths := setupCardIntegrityTest(t)
			parent := addIntegrityCard(t, svc, "Parent", "")
			var child *model.Card
			if strings.HasPrefix(operation, "edit") || operation == "update" {
				child = addIntegrityCard(t, svc, "Existing", parent.ID)
			}
			parent.Parent = "new"
			if operation == "restore regenerated alias" {
				addIntegrityCard(t, svc, "New", "")
				parent.Parent = "new-2"
			}
			if err := svc.cardStore.Update("main", parent); err != nil {
				t.Fatal(err)
			}
			before := map[string][]byte{parent.ID: readCardBytes(t, paths, parent.ID)}
			if child != nil {
				before[child.ID] = readCardBytes(t, paths, child.ID)
			}
			var err error
			switch operation {
			case "add":
				_, _, err = svc.Add(AddCardInput{BoardName: "main", Title: "New", Parent: parent.ID})
			case "restore", "restore regenerated alias":
				err = svc.Restore("main", &model.Card{ID: "restored", Alias: "new", Title: "New", Parent: parent.ID}, "backlog", -1)
			case "edit title":
				title, column := "New", "done"
				_, err = svc.Edit(EditCardInput{BoardName: "main", CardIDOrAlias: child.ID, Title: &title, Column: &column})
			case "edit alias":
				alias, column := "new", "done"
				_, err = svc.Edit(EditCardInput{BoardName: "main", CardIDOrAlias: child.ID, Alias: &alias, Column: &column})
			case "update":
				child.Alias = "new"
				err = svc.Update("main", child)
			}
			if err == nil || !strings.Contains(err.Error(), "parent cycle") || !strings.Contains(err.Error(), parent.ID) {
				t.Fatalf("expected cycle through final alias, got %v", err)
			}
			for id, data := range before {
				if !bytes.Equal(data, readCardBytes(t, paths, id)) {
					t.Errorf("rejected cycle changed file %s", id)
				}
			}
			cards, listErr := svc.cardStore.List("main")
			wantCount := len(before)
			if operation == "restore regenerated alias" {
				wantCount++
			}
			if listErr != nil || len(cards) != wantCount {
				t.Fatalf("rejected cycle left an extra card: %+v, %v", cards, listErr)
			}
		})
	}
}

func TestCardService_AliasChangeCanBreakLegacyCycle(t *testing.T) {
	svc, _ := setupCardIntegrityTest(t)
	parent := addIntegrityCard(t, svc, "Parent", "")
	child := addIntegrityCard(t, svc, "Child", parent.ID)
	parent.Parent = child.Alias
	if err := svc.cardStore.Update("main", parent); err != nil {
		t.Fatal(err)
	}
	title := "Renamed"
	if _, err := svc.Edit(EditCardInput{BoardName: "main", CardIDOrAlias: child.ID, Title: &title}); err != nil {
		t.Fatalf("alias change removes the legacy reference and should break the cycle: %v", err)
	}
}
