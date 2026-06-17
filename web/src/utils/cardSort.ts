import type { BoardConfig, Card } from '../api/types';
import {
  FIELD_TYPE_ENUM,
  FIELD_TYPE_ENUM_SET,
  FIELD_TYPE_FREE_SET,
  FIELD_TYPE_BOOLEAN,
} from '../api/types';

// Card sorting for the board view. This mirrors model.SortCardsByField in
// internal/model/sort.go - keep the two in sync. It is a non-destructive view
// sort: it returns a reordered copy and never touches card positions.
//
// Ordering rules by field type:
//   - enum: by the option order defined in the board config (first option ranks
//     lowest); values not in the options list rank after all known options.
//   - enum-set: by the card's highest-ranked (lowest option index) member.
//   - free-set: by the card's alphabetically-first member.
//   - string / date: case-insensitive, then case-sensitive; ISO dates sort
//     chronologically as strings.
//   - boolean: false before true.
//
// Cards with no value for the field always sort to the end, in both directions.
// Cards that compare equal keep their existing order (position, then id), so the
// incoming manual order is preserved within a value group.

// Separator used to join set values into a single comparable string for the
// enum-set tiebreak. A NUL can't appear in field values, so it avoids spurious
// collisions (e.g. ["a","bc"] vs ["ab","c"]). Matches the Go implementation.
const SET_JOIN_SEP = String.fromCharCode(0);

/** Returns the custom field names that can be sorted on (all of them). */
export function sortableFieldNames(board: BoardConfig): string[] {
  return Object.keys(board.custom_fields ?? {});
}

/**
 * Resolves the sort field to honor for a board: the given field if the board
 * actually defines it, otherwise '' (manual order). A field may be stale - e.g.
 * carried in the URL from another board, or removed from config - so callers
 * funnel the raw value through here rather than trusting it directly.
 */
export function resolveSortField(board: BoardConfig, sortField: string): string {
  return sortField && board.custom_fields?.[sortField] ? sortField : '';
}

/**
 * Groups cards by column name, ordering each column by the active custom-field
 * sort and falling back to the incoming (manual) order. This is the single
 * source of truth shared by the board render and keyboard navigation, so the
 * two never disagree about ordering. Every column is returned, including empty
 * ones (callers that only want non-empty groups filter afterward).
 */
export function groupCardsByColumn(
  cards: Card[],
  board: BoardConfig,
  sortField: string,
  descending: boolean
): Record<string, Card[]> {
  const activeSort = resolveSortField(board, sortField);
  const groups: Record<string, Card[]> = {};
  for (const column of board.columns) {
    const colCards = cards.filter((c) => c.column === column.name);
    groups[column.name] = activeSort ? sortCards(colCards, board, activeSort, descending) : colCards;
  }
  return groups;
}

/** Returns a sorted copy of cards. An empty field returns the input order. */
export function sortCards(
  cards: Card[],
  board: BoardConfig,
  field: string,
  descending: boolean
): Card[] {
  if (!field) return cards;
  const cmp = fieldValueComparator(board, field);

  return [...cards].sort((a, b) => {
    const [va, hasA] = fieldSortValue(a, field);
    const [vb, hasB] = fieldSortValue(b, field);

    // Present values always sort before missing ones, in both directions.
    if (hasA !== hasB) return hasA ? -1 : 1;

    if (hasA && hasB) {
      let c = cmp(va, vb);
      if (descending) c = -c;
      if (c !== 0) return c;
    }

    // Equal values (or both unset): preserve manual order.
    const pa = a.position ?? '';
    const pb = b.position ?? '';
    if (pa !== pb) return pa < pb ? -1 : 1;
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
  });
}

// fieldSortValue returns a card's raw value and whether it counts as "set".
// Empty strings and empty arrays are unset; a boolean is set whenever present.
function fieldSortValue(card: Card, field: string): [unknown, boolean] {
  const v = card[field];
  if (v === undefined || v === null) return [v, false];
  if (typeof v === 'string') return [v, v !== ''];
  if (Array.isArray(v)) return [v, v.length > 0];
  return [v, true];
}

type Comparator = (a: unknown, b: unknown) => number;

function fieldValueComparator(board: BoardConfig, field: string): Comparator {
  const schema = board.custom_fields?.[field];
  const type = schema?.type;
  const options = schema?.options ?? [];

  switch (type) {
    case FIELD_TYPE_ENUM: {
      const ranks = optionRanks(options);
      return (a, b) => compareEnum(asString(a), asString(b), ranks, options.length);
    }
    case FIELD_TYPE_ENUM_SET: {
      const ranks = optionRanks(options);
      const n = options.length;
      return (a, b) => {
        const ra = minOptionRank(a, ranks, n);
        const rb = minOptionRank(b, ranks, n);
        if (ra !== rb) return ra - rb;
        return compareRaw(joinSortedSet(a), joinSortedSet(b));
      };
    }
    case FIELD_TYPE_BOOLEAN:
      return (a, b) => {
        const ba = a === true;
        const bb = b === true;
        if (ba === bb) return 0;
        return ba ? 1 : -1; // false before true
      };
    case FIELD_TYPE_FREE_SET:
      return (a, b) => compareStrings(minSetValue(a), minSetValue(b));
    default: // string, date, or unknown
      return (a, b) => compareStrings(asString(a), asString(b));
  }
}

function optionRanks(options: { value: string }[]): Map<string, number> {
  const ranks = new Map<string, number>();
  options.forEach((opt, i) => ranks.set(opt.value, i));
  return ranks;
}

function compareEnum(
  a: string,
  b: string,
  ranks: Map<string, number>,
  unknownRank: number
): number {
  const ra = ranks.get(a) ?? unknownRank;
  const rb = ranks.get(b) ?? unknownRank;
  if (ra !== rb) return ra - rb;
  return compareRaw(a, b);
}

function minOptionRank(v: unknown, ranks: Map<string, number>, unknownRank: number): number {
  let best = unknownRank + 1; // sentinel above any real or unknown rank
  for (const s of asStringArray(v)) {
    const r = ranks.get(s) ?? unknownRank;
    if (r < best) best = r;
  }
  return best;
}

function minSetValue(v: unknown): string {
  let min = '';
  let first = true;
  for (const s of asStringArray(v)) {
    if (first || s < min) {
      min = s;
      first = false;
    }
  }
  return min;
}

function joinSortedSet(v: unknown): string {
  return [...asStringArray(v)].sort().join(SET_JOIN_SEP);
}

// Case-insensitive first, then case-sensitive, so values differing only in case
// remain deterministically ordered.
function compareStrings(a: string, b: string): number {
  const c = compareRaw(a.toLowerCase(), b.toLowerCase());
  if (c !== 0) return c;
  return compareRaw(a, b);
}

// Byte-wise comparison matching Go's strings.Compare (not locale-aware), so the
// frontend and backend agree on ordering.
function compareRaw(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

function asString(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

function asStringArray(v: unknown): string[] {
  if (!Array.isArray(v)) return [];
  return v.filter((item): item is string => typeof item === 'string');
}
