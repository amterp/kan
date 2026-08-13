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
  if (position !== undefined && position >= 0 && position < targetColumnLength) {
    return position;
  }
  if (position !== undefined) {
    return targetColumnLength;
  }
  if (!isColumnChange) {
    return currentIndex >= 0 ? currentIndex : targetColumnLength;
  }
  const column = board.columns.find((c) => c.name === targetColumn);
  return moveInsertsAtTop(column) ? 0 : targetColumnLength;
}
