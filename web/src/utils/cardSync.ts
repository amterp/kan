import type { Card } from '../api/types';

/**
 * Places a card into the board's card array at its server-authoritative rank,
 * replacing any earlier copy of it.
 *
 * The array *is* the render order for manual (unsorted) boards - groupCardsByColumn
 * only filters by column - so a card whose position changed has to physically move
 * within the array. Updating it where it sits would leave a card reordered from the
 * CLI, or by another browser tab, sitting at its old rank until the next reload.
 *
 * Ordering mirrors the server's List (cardsInColumn in
 * internal/service/card_service.go): position ascending, ties broken by id
 * ascending. Keeping the two in agreement means a later full refresh never
 * reshuffles what we placed here.
 *
 * Only same-column neighbors are consulted, so a card can land at the very end of
 * the flat array even though its column sits earlier on the board. That is fine and
 * intentional: the array is grouped by column at render time, never read as
 * column-major.
 */
export function upsertCard(cards: Card[], card: Card): Card[] {
  const rest = cards.filter((c) => c.id !== card.id);

  for (let i = 0; i < rest.length; i++) {
    const other = rest[i];
    if (other.column !== card.column) continue;
    if (sortsAfter(other, card)) {
      return [...rest.slice(0, i), card, ...rest.slice(i)];
    }
  }

  return [...rest, card];
}

/** Whether `other` ranks below `card` in their shared column. */
function sortsAfter(other: Card, card: Card): boolean {
  const otherPos = other.position ?? '';
  const cardPos = card.position ?? '';
  if (otherPos !== cardPos) return otherPos > cardPos;
  return other.id > card.id;
}
