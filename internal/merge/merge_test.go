package merge

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/version"
)

func marshal(t *testing.T, c model.Card) []byte {
	t.Helper()
	b, err := c.MarshalFile()
	if err != nil {
		t.Fatalf("MarshalFile: %v", err)
	}
	return b
}

func mergeCards(t *testing.T, base, ours, theirs model.Card) (model.Card, []Conflict) {
	t.Helper()
	out, conflicts, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7)
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if len(conflicts) > 0 {
		return model.Card{}, conflicts // result has markers; caller asserts separately
	}
	var got model.Card
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("merged output is not valid JSON: %v\n%s", err, out)
	}
	return got, nil
}

// baseCard is a minimal, well-formed starting point for tests.
func baseCard() model.Card {
	return model.Card{
		Version:         3,
		ID:              "c_1",
		Alias:           "card",
		Title:           "Title",
		Creator:         "alice",
		CreatedAtMillis: 1000,
		UpdatedAtMillis: 1000,
		Column:          "Backlog",
		Position:        "m",
		History:         []model.HistoryEntry{{Field: "column", Value: "Backlog", At: 1000}},
	}
}

func TestCards_DifferentFieldsSameCard_AutoMerges(t *testing.T) {
	base := baseCard()

	ours := base
	ours.Title = "New title"
	ours.UpdatedAtMillis = 2000

	theirs := base
	theirs.Description = "New description"
	theirs.UpdatedAtMillis = 3000

	got, conflicts := mergeCards(t, base, ours, theirs)
	if conflicts != nil {
		t.Fatalf("expected clean merge, got conflicts %v", conflicts)
	}
	if got.Title != "New title" {
		t.Errorf("title = %q, want %q", got.Title, "New title")
	}
	if got.Description != "New description" {
		t.Errorf("description = %q, want %q", got.Description, "New description")
	}
	if got.UpdatedAtMillis != 3000 {
		t.Errorf("updated_at = %d, want max 3000", got.UpdatedAtMillis)
	}
}

func TestCards_UpdatedAtTakesMax(t *testing.T) {
	base := baseCard()
	ours := base
	ours.Title = "O"
	ours.UpdatedAtMillis = 5000
	theirs := base
	theirs.Description = "T"
	theirs.UpdatedAtMillis = 4000

	got, _ := mergeCards(t, base, ours, theirs)
	if got.UpdatedAtMillis != 5000 {
		t.Errorf("updated_at = %d, want 5000", got.UpdatedAtMillis)
	}
}

func TestCards_CreatedAtTakesMin(t *testing.T) {
	base := baseCard()
	ours := base
	ours.CreatedAtMillis = 900
	ours.Title = "O"
	ours.UpdatedAtMillis = 2000
	theirs := base
	theirs.CreatedAtMillis = 1000
	theirs.Description = "T"
	theirs.UpdatedAtMillis = 2001

	got, _ := mergeCards(t, base, ours, theirs)
	if got.CreatedAtMillis != 900 {
		t.Errorf("created_at = %d, want min 900", got.CreatedAtMillis)
	}
}

func TestCards_BothMoveSameCard_LastWriterWins(t *testing.T) {
	base := baseCard()

	ours := base
	ours.Column = "Doing"
	ours.Position = "n"
	ours.UpdatedAtMillis = 2000
	ours.History = append(append([]model.HistoryEntry{}, base.History...),
		model.HistoryEntry{Field: "column", Value: "Doing", At: 2000})

	theirs := base
	theirs.Column = "Done"
	theirs.Position = "p"
	theirs.UpdatedAtMillis = 3000
	theirs.History = append(append([]model.HistoryEntry{}, base.History...),
		model.HistoryEntry{Field: "column", Value: "Done", At: 3000})

	got, conflicts := mergeCards(t, base, ours, theirs)
	if conflicts != nil {
		t.Fatalf("expected clean merge, got conflicts %v", conflicts)
	}
	if got.Column != "Done" || got.Position != "p" {
		t.Errorf("placement = (%q,%q), want later (Done,p)", got.Column, got.Position)
	}
	// History should retain both moves plus the seed.
	if len(got.History) != 3 {
		t.Errorf("history len = %d, want 3 (union of both moves + seed)", len(got.History))
	}
}

func TestCards_OnlyOneSideMoves(t *testing.T) {
	base := baseCard()
	ours := base // unchanged placement
	ours.Title = "edited"
	ours.UpdatedAtMillis = 5000
	theirs := base
	theirs.Column = "Done"
	theirs.Position = "p"
	theirs.UpdatedAtMillis = 2000

	got, _ := mergeCards(t, base, ours, theirs)
	if got.Column != "Done" || got.Position != "p" {
		t.Errorf("placement = (%q,%q), want theirs (Done,p) even though ours is newer", got.Column, got.Position)
	}
	if got.Title != "edited" {
		t.Errorf("title = %q, want edited", got.Title)
	}
}

func TestCards_HistoryUnionDedupAndSort(t *testing.T) {
	base := baseCard()
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.History = []model.HistoryEntry{
		{Field: "column", Value: "Backlog", At: 1000},
		{Field: "column", Value: "Doing", At: 2000},
	}
	theirs := base
	theirs.UpdatedAtMillis = 1500
	theirs.History = []model.HistoryEntry{
		{Field: "column", Value: "Backlog", At: 1000},
		{Field: "column", Value: "Review", At: 1500},
	}

	got, _ := mergeCards(t, base, ours, theirs)
	want := []int64{1000, 1500, 2000}
	if len(got.History) != len(want) {
		t.Fatalf("history len = %d, want %d: %+v", len(got.History), len(want), got.History)
	}
	for i, at := range want {
		if got.History[i].At != at {
			t.Errorf("history[%d].At = %d, want %d (sorted, deduped)", i, got.History[i].At, at)
		}
	}
}

func TestCards_CommentsUnion(t *testing.T) {
	base := baseCard()
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.Comments = []model.Comment{{ID: "m1", Body: "from ours", Author: "alice", CreatedAtMillis: 2000}}
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.Comments = []model.Comment{{ID: "m2", Body: "from theirs", Author: "bob", CreatedAtMillis: 3000}}

	got, _ := mergeCards(t, base, ours, theirs)
	if len(got.Comments) != 2 {
		t.Fatalf("comments len = %d, want 2 (union): %+v", len(got.Comments), got.Comments)
	}
}

func TestCards_CommentEditLastWriterWins(t *testing.T) {
	base := baseCard()
	base.Comments = []model.Comment{{ID: "m1", Body: "original", Author: "alice", CreatedAtMillis: 1000}}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.Comments = []model.Comment{{ID: "m1", Body: "ours edit", Author: "alice", CreatedAtMillis: 1000, UpdatedAtMillis: 2000}}
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.Comments = []model.Comment{{ID: "m1", Body: "theirs edit", Author: "alice", CreatedAtMillis: 1000, UpdatedAtMillis: 3000}}

	got, _ := mergeCards(t, base, ours, theirs)
	if len(got.Comments) != 1 {
		t.Fatalf("comments len = %d, want 1", len(got.Comments))
	}
	if got.Comments[0].Body != "theirs edit" {
		t.Errorf("comment body = %q, want later edit %q", got.Comments[0].Body, "theirs edit")
	}
}

func TestCards_CommentDeletedOneSide(t *testing.T) {
	base := baseCard()
	base.Comments = []model.Comment{{ID: "m1", Body: "x", Author: "alice", CreatedAtMillis: 1000}}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.Comments = nil // deleted in ours
	theirs := base      // unchanged
	theirs.UpdatedAtMillis = 1500

	got, _ := mergeCards(t, base, ours, theirs)
	if len(got.Comments) != 0 {
		t.Errorf("comments len = %d, want 0 (deletion honored)", len(got.Comments))
	}
}

func TestCards_SetCustomFieldUnion(t *testing.T) {
	base := baseCard()
	base.CustomFields = map[string]any{"tags": []any{"a"}}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.CustomFields = map[string]any{"tags": []any{"a", "b"}}
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.CustomFields = map[string]any{"tags": []any{"a", "c"}}

	got, _ := mergeCards(t, base, ours, theirs)
	tags, ok := got.CustomFields["tags"].([]any)
	if !ok {
		t.Fatalf("tags missing or wrong type: %#v", got.CustomFields["tags"])
	}
	want := map[string]bool{"a": true, "b": true, "c": true}
	if len(tags) != 3 {
		t.Fatalf("tags = %v, want union of 3", tags)
	}
	for _, v := range tags {
		delete(want, v.(string))
	}
	if len(want) != 0 {
		t.Errorf("tags = %v, missing %v", tags, want)
	}
}

func TestCards_SetCustomFieldRemovalHonored(t *testing.T) {
	base := baseCard()
	base.CustomFields = map[string]any{"tags": []any{"a", "b"}}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.CustomFields = map[string]any{"tags": []any{"a"}} // removed b
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.CustomFields = map[string]any{"tags": []any{"a", "b", "c"}} // added c

	got, _ := mergeCards(t, base, ours, theirs)
	tags := got.CustomFields["tags"].([]any)
	for _, v := range tags {
		if v.(string) == "b" {
			t.Errorf("tags = %v, expected b removed", tags)
		}
	}
}

func TestCards_ScalarCustomFieldOneSideChanged(t *testing.T) {
	base := baseCard()
	base.CustomFields = map[string]any{"status": "todo"}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.CustomFields = map[string]any{"status": "doing"}
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.CustomFields = map[string]any{"status": "todo"} // unchanged

	got, conflicts := mergeCards(t, base, ours, theirs)
	if conflicts != nil {
		t.Fatalf("expected clean merge, got %v", conflicts)
	}
	if got.CustomFields["status"] != "doing" {
		t.Errorf("status = %v, want doing", got.CustomFields["status"])
	}
}

func TestCards_TitleConflictSurfaced(t *testing.T) {
	base := baseCard()
	ours := base
	ours.Title = "Ours title"
	ours.UpdatedAtMillis = 2000
	theirs := base
	theirs.Title = "Theirs title"
	theirs.UpdatedAtMillis = 3000

	out, conflicts, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7)
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].Field != "title" {
		t.Fatalf("conflicts = %v, want [title]", conflicts)
	}
	if conflicts[0].Ours != "Ours title" || conflicts[0].Theirs != "Theirs title" {
		t.Errorf("conflict values = %+v, want ours/theirs titles", conflicts[0])
	}
	if !bytes.Contains(out, []byte("<<<<<<<")) || !bytes.Contains(out, []byte(">>>>>>>")) {
		t.Errorf("expected git conflict markers in output:\n%s", out)
	}
	if !bytes.Contains(out, []byte("Ours title")) || !bytes.Contains(out, []byte("Theirs title")) {
		t.Errorf("expected both candidate titles in output:\n%s", out)
	}
}

func TestCards_ScalarCustomFieldConflictSurfaced(t *testing.T) {
	base := baseCard()
	base.CustomFields = map[string]any{"status": "todo"}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.CustomFields = map[string]any{"status": "doing"}
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.CustomFields = map[string]any{"status": "done"}

	_, conflicts, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7)
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].Field != "custom:status" {
		t.Fatalf("conflicts = %v, want [custom:status]", conflicts)
	}
}

func TestCards_IdenticalSidesNoConflict(t *testing.T) {
	base := baseCard()
	ours := base
	ours.Title = "same"
	ours.UpdatedAtMillis = 2000
	theirs := ours

	got, conflicts := mergeCards(t, base, ours, theirs)
	if conflicts != nil {
		t.Fatalf("identical edits should not conflict, got %v", conflicts)
	}
	if got.Title != "same" {
		t.Errorf("title = %q, want same", got.Title)
	}
}

func TestCards_EmptyBaseAddAdd_Identical(t *testing.T) {
	card := baseCard()
	card.UpdatedAtMillis = 2000
	out, conflicts, err := Cards(nil, marshal(t, card), marshal(t, card), 7)
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if conflicts != nil {
		t.Fatalf("identical add/add should not conflict, got %v", conflicts)
	}
	var got model.Card
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if got.ID != card.ID {
		t.Errorf("id = %q, want %q", got.ID, card.ID)
	}
}

func TestCards_EmptySideErrors(t *testing.T) {
	base := baseCard()
	_, _, err := Cards(marshal(t, base), nil, marshal(t, base), 7)
	if err == nil {
		t.Error("expected error when a side is empty (delete/modify)")
	}
}

// TestCards_CustomFieldTypeChangeTakesChangedSide is the direct regression for
// the P0 silent-data-loss bug: when one side leaves an array field untouched and
// the other changes it to a scalar, the changed value must win cleanly - not be
// nulled out by routing a scalar through the set-union path.
func TestCards_CustomFieldTypeChangeTakesChangedSide(t *testing.T) {
	base := baseCard()
	base.CustomFields = map[string]any{"tags": []any{"a", "b"}}
	ours := base // tags untouched (still the base array)
	ours.UpdatedAtMillis = 2000
	ours.Title = "ours edit"
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.CustomFields = map[string]any{"tags": "single"} // array -> scalar

	got, conflicts := mergeCards(t, base, ours, theirs)
	if conflicts != nil {
		t.Fatalf("only theirs changed the field; expected clean take-theirs, got %v", conflicts)
	}
	if got.CustomFields["tags"] != "single" {
		t.Errorf("tags = %#v, want theirs' scalar \"single\" (must not be lost to null)", got.CustomFields["tags"])
	}
}

// TestCards_CustomFieldTypeMismatchBothChanged surfaces a conflict (never a
// silent merge) when both sides change a field to incompatible types.
func TestCards_CustomFieldTypeMismatchBothChanged(t *testing.T) {
	base := baseCard()
	base.CustomFields = map[string]any{"tags": []any{"a", "b"}}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.CustomFields = map[string]any{"tags": []any{"a", "b", "c"}} // still array, changed
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.CustomFields = map[string]any{"tags": "single"} // changed to scalar

	out, conflicts, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7)
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].Field != "custom:tags" {
		t.Fatalf("conflicts = %v, want [custom:tags] (type mismatch must surface, not merge)", conflicts)
	}
	if bytes.Contains(out, []byte(": null")) {
		t.Errorf("type-mismatch merge wrote a null (data loss):\n%s", out)
	}
}

// TestCards_EmptiedSetSerializesAsEmptyArray guards that a set unioned down to
// nothing is written as [] (a cleared set), never null.
func TestCards_EmptiedSetSerializesAsEmptyArray(t *testing.T) {
	base := baseCard()
	base.CustomFields = map[string]any{"tags": []any{"a", "b"}}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.CustomFields = map[string]any{"tags": []any{"b"}} // removed a
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.CustomFields = map[string]any{"tags": []any{"a"}} // removed b

	out, conflicts, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7)
	if err != nil || conflicts != nil {
		t.Fatalf("Cards: err=%v conflicts=%v", err, conflicts)
	}
	var got model.Card
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	tags, ok := got.CustomFields["tags"].([]any)
	if !ok {
		t.Fatalf("tags = %#v, want empty []any (not null)", got.CustomFields["tags"])
	}
	if len(tags) != 0 {
		t.Errorf("tags = %v, want empty", tags)
	}
}

// TestCards_SetUnionOverCapConflicts guards that a set union exceeding the
// per-field cap surfaces a conflict rather than silently overflowing (which the
// CLI would later reject) or truncating (which would silently drop items).
func TestCards_SetUnionOverCapConflicts(t *testing.T) {
	base := baseCard()
	baseTags := []any{"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7"} // 8 items
	base.CustomFields = map[string]any{"tags": baseTags}
	ours := base
	ours.UpdatedAtMillis = 2000
	ours.CustomFields = map[string]any{"tags": append(append([]any{}, baseTags...), "o1", "o2")} // 10
	theirs := base
	theirs.UpdatedAtMillis = 3000
	theirs.CustomFields = map[string]any{"tags": append(append([]any{}, baseTags...), "x1", "x2")} // 10; union -> 12

	_, conflicts, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7)
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].Field != "custom:tags" {
		t.Fatalf("conflicts = %v, want [custom:tags] (union over MaxSetItems must surface)", conflicts)
	}
}

// TestCards_SymmetricUnderSwap is the rebase-safety property: a divergence whose
// every LWW path lands on an exact-timestamp tie must merge to byte-identical
// output regardless of which side is "ours". Rebase and cherry-pick swap the
// sides, so without deterministic tiebreaks the two collaborators' files would
// silently diverge. This one scenario exercises placement, alias, comment, and
// history ties simultaneously.
func TestCards_SymmetricUnderSwap(t *testing.T) {
	base := baseCard()

	ours := base
	ours.UpdatedAtMillis = 2000
	ours.Column, ours.Position = "Doing", "n"
	ours.Alias, ours.AliasExplicit = "alias-ours", true
	ours.History = append(append([]model.HistoryEntry{}, base.History...),
		model.HistoryEntry{Field: "column", Value: "Doing", At: 2000})
	ours.Comments = []model.Comment{{ID: "m1", Body: "ours", Author: "alice", CreatedAtMillis: 1500, UpdatedAtMillis: 2000}}

	theirs := base
	theirs.UpdatedAtMillis = 2000 // exact tie with ours on every LWW path
	theirs.Column, theirs.Position = "Done", "p"
	theirs.Alias, theirs.AliasExplicit = "alias-theirs", true
	theirs.History = append(append([]model.HistoryEntry{}, base.History...),
		model.HistoryEntry{Field: "column", Value: "Done", At: 2000})
	theirs.Comments = []model.Comment{{ID: "m1", Body: "theirs", Author: "bob", CreatedAtMillis: 1500, UpdatedAtMillis: 2000}}

	fwd, cf1, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7)
	if err != nil || cf1 != nil {
		t.Fatalf("forward merge: err=%v conflicts=%v", err, cf1)
	}
	rev, cf2, err := Cards(marshal(t, base), marshal(t, theirs), marshal(t, ours), 7)
	if err != nil || cf2 != nil {
		t.Fatalf("reverse merge: err=%v conflicts=%v", err, cf2)
	}
	if !bytes.Equal(fwd, rev) {
		t.Errorf("merge not symmetric under ours/theirs swap (rebase would diverge):\nforward:\n%s\nreverse:\n%s", fwd, rev)
	}
}

// TestCards_FutureSchemaRefused guards that the driver refuses to merge a card
// migrated to a schema this binary doesn't understand, rather than half-merging
// it and re-stamping the future version.
func TestCards_FutureSchemaRefused(t *testing.T) {
	base := baseCard()
	ours := base
	ours.Title = "ours"
	ours.UpdatedAtMillis = 2000
	theirs := base
	theirs.Version = version.CurrentCardVersion + 1 // a newer kan migrated it
	theirs.Description = "theirs"
	theirs.UpdatedAtMillis = 3000

	if _, _, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7); err == nil {
		t.Fatal("expected refusal to merge a future-schema card, got nil error")
	}
}

func TestCards_OutputMatchesOnDiskFormat(t *testing.T) {
	// A clean merge should produce byte-identical output to MarshalFile so the
	// driver's writes look exactly like Kan's own writes (clean diffs).
	base := baseCard()
	ours := base
	ours.Title = "edited"
	ours.UpdatedAtMillis = 2000
	theirs := base
	theirs.UpdatedAtMillis = 2000 // no real change beyond ours' title

	out, conflicts, err := Cards(marshal(t, base), marshal(t, ours), marshal(t, theirs), 7)
	if err != nil || conflicts != nil {
		t.Fatalf("Cards: err=%v conflicts=%v", err, conflicts)
	}
	var merged model.Card
	if err := json.Unmarshal(out, &merged); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	want := marshal(t, merged)
	if !bytes.Equal(out, want) {
		t.Errorf("output not in canonical on-disk format:\ngot:\n%s\nwant:\n%s", out, want)
	}
}
