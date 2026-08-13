import type { BoardConfig, Column } from '../api/types';

// Where a card lands when it is moved into a column. Mirrors the server-side rule
// in CardService.MoveCardWithPlacement (internal/service/card_service.go) - keep
// the two in sync, or the optimistic update will render the card in one place
// and the next WebSocket refresh will visibly jump it somewhere else.

/** Unset means top: a card arriving in a column is usually the one you just acted on. */
export function moveInsertsAtTop(column: Column | undefined): boolean {
  return column?.on_move_default_position !== 'bottom';
}

/**
 * The one spelling of "the end of the column" that both sides read the same way:
 * the server's computePosition counts negatives back from the end, and
 * resolveInsertIndex below appends. Sending a length instead would be wrong under
 * a filter, where the visible column is shorter than the real one.
 *
 * Pass this rather than omitting the position when the caller means the bottom.
 * An omitted position is not "the end" - it hands the choice to the column's
 * on_move_default_position, which lands the card at the top of most columns.
 */
export const APPEND_POSITION = -1;

/**
 * Index at which a moved card should be spliced into the destination column,
 * mirroring the server. `position` is the explicit placement the caller
 * requested, if any; when undefined the column's on_move_default_position decides.
 *
 * `currentIndex` is where the card sits in that column today, used only for the
 * same-column case: a bare move within a column is a server-side no-op, so the
 * card must stay put rather than being shuffled to an end.
 */
export function resolveInsertIndex(
  board: BoardConfig,
  targetColumn: string,
  targetColumnLength: number,
  position: number | undefined,
  isColumnChange: boolean,
  currentIndex: number,
): number {
  if (position !== undefined) {
    return normalizeIndex(position, targetColumnLength);
  }
  if (!isColumnChange) {
    return currentIndex >= 0 ? currentIndex : targetColumnLength;
  }
  const column = board.columns.find((c) => c.name === targetColumn);
  return moveInsertsAtTop(column) ? 0 : targetColumnLength;
}

/**
 * Splice index for an explicit position, matching computePosition on the server:
 * negatives count back from the end (-1 = after the last card, -2 = before it),
 * underflow clamps to the top, and anything past the end appends.
 */
function normalizeIndex(position: number, length: number): number {
  if (position < 0) {
    return Math.max(0, length + 1 + position);
  }
  return Math.min(position, length);
}
