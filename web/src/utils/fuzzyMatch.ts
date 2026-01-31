import type { Card, BoardConfig } from '../api/types';

/**
 * Substring match: query must appear as a consecutive substring in target.
 * Case-insensitive.
 *
 * Examples:
 *   fuzzyMatch("bug", "fixing a bug") → true
 *   fuzzyMatch("fix", "prefix") → true
 *   fuzzyMatch("fg", "fixing a bug") → false (not consecutive)
 */
export function fuzzyMatch(query: string, target: string): boolean {
  return target.toLowerCase().includes(query.toLowerCase());
}

/**
 * Check if a card matches a search query.
 * Searches: title, alias, description, and all custom field values.
 *
 * Query is split on whitespace into words. Each word must appear as a
 * consecutive substring (case-insensitive) in at least one field.
 * All words must match (AND logic), but can match different fields.
 *
 * Examples with query "fix bug":
 *   - "Bug fix for login" (title) → match
 *   - "Fixing a nasty bug" (title) → match
 *   - Title: "Fix login", Description: "Related to bug #123" → match
 *   - "f-i-x b-u-g" → no match
 */
export function cardMatchesQuery(card: Card, query: string, board: BoardConfig): boolean {
  const words = query.toLowerCase().split(/\s+/).filter((w) => w.length > 0);
  if (words.length === 0) return true;

  // Build searchable texts from all fields
  const searchableTexts: string[] = [
    card.title.toLowerCase(),
    card.alias.toLowerCase(),
    card.description?.toLowerCase() ?? '',
  ];

  // Add custom field values
  if (board.custom_fields) {
    for (const fieldName of Object.keys(board.custom_fields)) {
      const value = card[fieldName];
      if (value == null) continue;
      if (Array.isArray(value)) {
        searchableTexts.push(...value.map((v) => String(v).toLowerCase()));
      } else {
        searchableTexts.push(String(value).toLowerCase());
      }
    }
  }

  // Each word must appear in at least one field
  return words.every((word) => searchableTexts.some((text) => text.includes(word)));
}
