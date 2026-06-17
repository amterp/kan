// Package merge performs semantic, field-aware 3-way merges of Kan card files.
//
// Git merges files line by line and has no idea what a card means, so two
// people touching the same card - even in unrelated fields - collide on shared
// lines like updated_at_millis. Kan understands the format, so it can resolve
// most of these conflicts correctly: take the later timestamp, union the
// append-only history and comment lists, let the last writer win a move. Only
// genuinely conflicting edits (two different rewrites of the same free-text
// field) are surfaced for a human, via standard git conflict markers.
//
// The guardrail: never silently drop a free-text edit. Timestamps, history,
// comments, sets and placement have safe resolutions; title/description and
// scalar custom fields fall back to a conflict when both sides diverge.
package merge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/version"
)

const defaultMarkerLen = 7

// Conflict describes a single field that diverged on both sides and could not be
// resolved safely. Ours/Theirs are human-readable renderings of the two values,
// for display (e.g. a diff) - the resolution itself happens via conflict markers
// in the returned bytes.
type Conflict struct {
	Field  string
	Ours   string
	Theirs string
}

// Cards performs a 3-way merge of card JSON: base is the common ancestor, ours
// is the current branch, theirs is the incoming branch. It returns the merged
// bytes ready to write to disk. When conflicts is non-empty the bytes contain
// git conflict markers and the caller should treat the merge as unresolved
// (the driver exits non-zero). markerLen is git's conflict-marker length (%L);
// a value <= 0 uses the git default of 7.
func Cards(base, ours, theirs []byte, markerLen int) (result []byte, conflicts []Conflict, err error) {
	if markerLen <= 0 {
		markerLen = defaultMarkerLen
	}
	if len(bytes.TrimSpace(ours)) == 0 || len(bytes.TrimSpace(theirs)) == 0 {
		return nil, nil, fmt.Errorf("cannot merge: a side is empty (delete/modify conflict)")
	}

	b, err := parseCard(base)
	if err != nil {
		return nil, nil, fmt.Errorf("parse base: %w", err)
	}
	o, err := parseCard(ours)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ours: %w", err)
	}
	t, err := parseCard(theirs)
	if err != nil {
		return nil, nil, fmt.Errorf("parse theirs: %w", err)
	}

	// Refuse to merge a card whose schema is newer than this binary understands.
	// Our field-by-field logic would mishandle fields it doesn't know and then
	// re-stamp the future version onto the result; failing to a conflict pushes
	// the user to upgrade kan instead. The store enforces the same strictness on
	// normal reads, but the driver writes straight to disk, so it must guard too.
	if version.IsFutureCardVersion(o.Version) || version.IsFutureCardVersion(t.Version) {
		return nil, nil, fmt.Errorf(
			"card schema newer than this kan understands (sides v%d/v%d, max v%d); upgrade kan",
			o.Version, t.Version, version.CurrentCardVersion)
	}

	m := model.Card{}
	var theirsOverrides []func(*model.Card)
	addConflict := func(c Conflict, applyTheirs func(*model.Card)) {
		conflicts = append(conflicts, c)
		theirsOverrides = append(theirsOverrides, applyTheirs)
	}

	// Schema version: take the newer (a migration may have bumped one side).
	m.Version = maxInt(o.Version, t.Version)

	// Identity fields never meaningfully diverge - prefer ours on any drift.
	m.ID = take3(b.ID, o.ID, t.ID)
	m.Creator = take3(b.Creator, o.Creator, t.Creator)

	// alias: last writer wins (a rename is a legitimate, low-stakes change).
	m.Alias, m.AliasExplicit = mergeAlias(b, o, t)

	// The card was "last updated" at the later of the two touches; "created" at
	// the earlier (these should never actually differ).
	m.UpdatedAtMillis = maxInt64(o.UpdatedAtMillis, t.UpdatedAtMillis)
	m.CreatedAtMillis = minNonZero(o.CreatedAtMillis, t.CreatedAtMillis)

	// Free text: auto-resolve when at most one side changed; conflict otherwise.
	m.Title = mergeText(b.Title, o.Title, t.Title, "title", addConflict)
	m.Description = mergeText(b.Description, o.Description, t.Description, "description", addConflict)
	m.Parent = mergeText(b.Parent, o.Parent, t.Parent, "parent", addConflict)

	// Column + position move as a unit; the last writer wins the placement.
	m.Column, m.Position = mergePlacement(b, o, t)

	// Append-only / set-like structures union cleanly.
	m.History = mergeHistory(b.History, o.History, t.History)
	m.Comments = mergeComments(b.Comments, o.Comments, t.Comments)

	// Custom fields: array values are sets (union); scalars are 3-way + conflict.
	m.CustomFields = mergeCustomFields(b.CustomFields, o.CustomFields, t.CustomFields, addConflict)
	if len(m.CustomFields) == 0 {
		m.CustomFields = nil
	}

	if len(conflicts) == 0 {
		out, err := m.MarshalFile()
		if err != nil {
			return nil, nil, fmt.Errorf("marshal merged card: %w", err)
		}
		return out, nil, nil
	}

	// Build the "theirs" candidate: the merged card with conflicting fields
	// swapped to theirs' values, then emit both inside conflict markers so the
	// human only has to resolve the field(s) that genuinely diverged.
	mt := m
	mt.CustomFields = cloneMap(m.CustomFields)
	for _, applyTheirs := range theirsOverrides {
		applyTheirs(&mt)
	}
	out, err := renderConflict(m, mt, markerLen)
	if err != nil {
		return nil, nil, err
	}
	return out, conflicts, nil
}

func parseCard(b []byte) (model.Card, error) {
	var c model.Card
	if len(bytes.TrimSpace(b)) == 0 {
		return c, nil // empty base (add/add conflict): treat as a zero card
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}

// take3 resolves an identity-like field, preferring ours when both sides drift.
func take3(base, ours, theirs string) string {
	switch ours {
	case theirs:
		return ours
	case base:
		return theirs
	default:
		return ours
	}
}

// mergeText is a standard 3-way string merge that records a conflict (and keeps
// ours as the placeholder value) when both sides change the field differently.
func mergeText(base, ours, theirs, field string, addConflict func(Conflict, func(*model.Card))) string {
	switch {
	case ours == theirs:
		return ours
	case ours == base:
		return theirs
	case theirs == base:
		return ours
	default:
		theirsVal := theirs
		addConflict(Conflict{Field: field, Ours: ours, Theirs: theirs}, textSetter(field, theirsVal))
		return ours
	}
}

func textSetter(field, val string) func(*model.Card) {
	switch field {
	case "title":
		return func(c *model.Card) { c.Title = val }
	case "description":
		return func(c *model.Card) { c.Description = val }
	case "parent":
		return func(c *model.Card) { c.Parent = val }
	default:
		// Programming error: a text field was wired into Cards() without a setter
		// here. Fail loud rather than silently dropping the theirs candidate.
		panic("merge: textSetter has no case for field " + field)
	}
}

func mergeAlias(b, o, t model.Card) (string, bool) {
	switch {
	case o.Alias == t.Alias:
		return o.Alias, o.AliasExplicit || t.AliasExplicit
	case o.Alias == b.Alias:
		return t.Alias, t.AliasExplicit
	case t.Alias == b.Alias:
		return o.Alias, o.AliasExplicit
	case t.UpdatedAtMillis != o.UpdatedAtMillis:
		if t.UpdatedAtMillis > o.UpdatedAtMillis {
			return t.Alias, t.AliasExplicit
		}
		return o.Alias, o.AliasExplicit
	default:
		// Exact-timestamp tie: pick deterministically by alias so the winner
		// doesn't depend on which side is "ours" (rebase swaps the sides).
		if t.Alias > o.Alias {
			return t.Alias, t.AliasExplicit
		}
		return o.Alias, o.AliasExplicit
	}
}

// mergePlacement resolves column + position together so they never desync. If
// both sides moved the card, the later edit (by card updated_at) wins.
func mergePlacement(b, o, t model.Card) (column, position string) {
	oursMoved := o.Column != b.Column || o.Position != b.Position
	theirsMoved := t.Column != b.Column || t.Position != b.Position
	switch {
	case oursMoved && theirsMoved:
		// Later move wins. On an exact-timestamp tie, break by the placement
		// itself so the result is independent of which side is "ours" - rebase
		// and cherry-pick swap ours/theirs, and the merge must still agree.
		if t.UpdatedAtMillis != o.UpdatedAtMillis {
			if t.UpdatedAtMillis > o.UpdatedAtMillis {
				return t.Column, t.Position
			}
			return o.Column, o.Position
		}
		if placementKey(t) > placementKey(o) {
			return t.Column, t.Position
		}
		return o.Column, o.Position
	case theirsMoved:
		return t.Column, t.Position
	default: // only ours moved, or neither
		return o.Column, o.Position
	}
}

func placementKey(c model.Card) string { return c.Column + "\x00" + c.Position }

// mergeHistory unions append-only history across all three versions, dedups by
// (field, value, at), and re-sorts chronologically. Because entries are
// immutable and append-only, the union is the correct 3-way result.
func mergeHistory(base, ours, theirs []model.HistoryEntry) []model.HistoryEntry {
	seen := map[string]bool{}
	var out []model.HistoryEntry
	add := func(entries []model.HistoryEntry) {
		for _, e := range entries {
			k := historyKey(e)
			if !seen[k] {
				seen[k] = true
				out = append(out, e)
			}
		}
	}
	add(base)
	add(ours)
	add(theirs)
	// Sort by event-time, then by a stable content key so entries sharing a
	// millisecond order identically regardless of merge direction (rebase swaps
	// ours/theirs, which would otherwise flip equal-At entries and leave the two
	// collaborators' files perpetually divergent).
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return historyKey(out[i]) < historyKey(out[j])
	})
	return out
}

func historyKey(e model.HistoryEntry) string {
	v, _ := json.Marshal(e.Value)
	return e.Field + "\x00" + strconv.FormatInt(e.At, 10) + "\x00" + string(v)
}

// mergeComments unions comments by id. A comment present on both sides resolves
// last-writer-wins by its own updated_at; a comment present in base but removed
// on one side is treated as deleted; new comments on either side are kept.
func mergeComments(base, ours, theirs []model.Comment) []model.Comment {
	bm := indexComments(base)
	om := indexComments(ours)
	tm := indexComments(theirs)

	var out []model.Comment
	for _, id := range orderedCommentIDs(base, ours, theirs) {
		bc, inB := bm[id]
		oc, inO := om[id]
		tc, inT := tm[id]
		switch {
		case inO && inT:
			out = append(out, lwwComment(oc, tc))
		case inO && !inT:
			if inB && eqAny(bc, oc) {
				continue // unchanged in ours, deleted in theirs
			}
			out = append(out, oc)
		case !inO && inT:
			if inB && eqAny(bc, tc) {
				continue // unchanged in theirs, deleted in ours
			}
			out = append(out, tc)
		}
	}
	return out
}

func indexComments(list []model.Comment) map[string]model.Comment {
	m := make(map[string]model.Comment, len(list))
	for _, c := range list {
		m[c.ID] = c
	}
	return m
}

func orderedCommentIDs(lists ...[]model.Comment) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range lists {
		for _, c := range list {
			if !seen[c.ID] {
				seen[c.ID] = true
				out = append(out, c.ID)
			}
		}
	}
	return out
}

func lwwComment(a, b model.Comment) model.Comment {
	ta, tb := commentTime(a), commentTime(b)
	if ta != tb {
		if tb > ta {
			return b
		}
		return a
	}
	// Exact-timestamp tie: pick deterministically by body so the winner doesn't
	// depend on which side is "ours" (rebase swaps the sides).
	if b.Body > a.Body {
		return b
	}
	return a
}

func commentTime(c model.Comment) int64 {
	if c.UpdatedAtMillis > c.CreatedAtMillis {
		return c.UpdatedAtMillis
	}
	return c.CreatedAtMillis
}

// mergeCustomFields merges board-defined fields. Array values are treated as
// sets and unioned; scalar values are a standard 3-way merge that conflicts
// when both sides change them differently (we can't tell free text from an
// enum here, so we choose safety over silent loss).
func mergeCustomFields(base, ours, theirs map[string]any, addConflict func(Conflict, func(*model.Card))) map[string]any {
	out := map[string]any{}
	// conflictField keeps ours as the in-place value and records a conflict whose
	// "theirs" candidate restores tv - shared by the type/value-conflict path and
	// the set-overflow path below.
	conflictField := func(k string, ov, tv any) {
		out[k] = ov
		key, theirsVal := k, tv
		addConflict(
			Conflict{Field: "custom:" + k, Ours: displayValue(ov), Theirs: displayValue(tv)},
			func(c *model.Card) {
				if c.CustomFields == nil {
					c.CustomFields = map[string]any{}
				}
				c.CustomFields[key] = theirsVal
			},
		)
	}
	for _, k := range orderedKeys(base, ours, theirs) {
		bv, inB := base[k]
		ov, inO := ours[k]
		tv, inT := theirs[k]
		switch {
		case inO && inT:
			switch {
			case eqAny(ov, tv):
				out[k] = ov
			case isArray(ov) && isArray(tv):
				merged := mergeSet(toArr(bv), toArr(ov), toArr(tv))
				if len(merged) > model.MaxSetItems {
					// The union overflows the per-field set cap. We won't silently
					// drop items, so surface it as a conflict for the user to trim.
					conflictField(k, ov, tv)
				} else {
					out[k] = merged
				}
			case inB && eqAny(ov, bv):
				out[k] = tv
			case inB && eqAny(tv, bv):
				out[k] = ov
			default:
				conflictField(k, ov, tv)
			}
		case inO && !inT:
			if inB && eqAny(ov, bv) {
				continue // deleted in theirs
			}
			out[k] = ov
		case !inO && inT:
			if inB && eqAny(tv, bv) {
				continue // deleted in ours
			}
			out[k] = tv
		}
	}
	return out
}

// mergeSet computes a 3-way set union: start from base minus anything either
// side removed, then add everything either side introduced.
func mergeSet(base, ours, theirs []any) []any {
	keyOf := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	asSet := func(list []any) map[string]bool {
		s := make(map[string]bool, len(list))
		for _, v := range list {
			s[keyOf(v)] = true
		}
		return s
	}
	oursS, theirsS := asSet(ours), asSet(theirs)
	removed := map[string]bool{}
	for _, v := range base {
		k := keyOf(v)
		if !oursS[k] || !theirsS[k] {
			removed[k] = true
		}
	}

	var out []any
	seen := map[string]bool{}
	add := func(list []any) {
		for _, v := range list {
			k := keyOf(v)
			if removed[k] || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, v)
		}
	}
	add(base)
	add(ours)
	add(theirs)
	if out == nil {
		// A fully-emptied set must serialize as an empty array, never null: the
		// rest of kan reads [] for a cleared set and would choke on a null.
		return []any{}
	}
	return out
}

func orderedKeys(maps ...map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range maps {
		var keys []string
		for k := range m {
			if !seen[k] {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys) // deterministic ordering for new keys in this map
		for _, k := range keys {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

func renderConflict(ours, theirs model.Card, markerLen int) ([]byte, error) {
	o, err := ours.MarshalFile()
	if err != nil {
		return nil, err
	}
	t, err := theirs.MarshalFile()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(strings.Repeat("<", markerLen) + " ours\n")
	buf.Write(o)
	buf.WriteByte('\n')
	buf.WriteString(strings.Repeat("=", markerLen) + "\n")
	buf.Write(t)
	buf.WriteByte('\n')
	buf.WriteString(strings.Repeat(">", markerLen) + " theirs\n")
	return buf.Bytes(), nil
}

func eqAny(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}

// displayValue renders a custom-field value for human-facing conflict output:
// strings verbatim, everything else as compact JSON.
func displayValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func isArray(v any) bool { _, ok := v.([]any); return ok }

func toArr(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minNonZero(a, b int64) int64 {
	switch {
	case a == 0:
		return b
	case b == 0:
		return a
	case a < b:
		return a
	default:
		return b
	}
}
