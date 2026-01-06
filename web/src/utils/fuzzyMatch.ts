import type { Card, BoardConfig } from '../api/types';

/**
 * Fuzzy match: all query characters must appear in target, in order, but not necessarily consecutively.
 * Case-insensitive.
 *
 * Examples:
 *   fuzzyMatch("abc", "aXbYc") → true (a...b...c in order)
 *   fuzzyMatch("usr", "user") → true
 *   fuzzyMatch("abc", "cab") → false (wrong order)
 */
export function fuzzyMatch(query: string, target: string): boolean {
  const q = query.toLowerCase();
  const t = target.toLowerCase();
  let qi = 0;
  for (let ti = 0; ti < t.length && qi < q.length; ti++) {
    if (t[ti] === q[qi]) qi++;
  }
  return qi === q.length;
}

/**
 * Check if a card matches a search query.
 * Searches: title, alias, description, and all custom field values.
 */
export function cardMatchesQuery(card: Card, query: string, board: BoardConfig): boolean {
  if (!query.trim()) return true;

  // Standard fields
  if (fuzzyMatch(query, card.title)) return true;
  if (fuzzyMatch(query, card.alias)) return true;
  if (card.description && fuzzyMatch(query, card.description)) return true;

  // Custom fields
  if (board.custom_fields) {
    for (const fieldName of Object.keys(board.custom_fields)) {
      const value = card[fieldName];
      if (value == null) continue;
      if (Array.isArray(value)) {
        if (value.some((v) => fuzzyMatch(query, String(v)))) return true;
      } else {
        if (fuzzyMatch(query, String(value))) return true;
      }
    }
  }
  return false;
}
