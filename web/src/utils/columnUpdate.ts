import type { BoardConfig, UpdateColumnInput } from '../api/types';

/**
 * Applies a column update to a board the way the server will, so the UI can
 * show it before the request returns. A rename also renames the column in
 * default_column and in ready.order, as BoardService does.
 */
export function applyColumnUpdate(
  board: BoardConfig,
  columnName: string,
  updates: UpdateColumnInput
): BoardConfig {
  const newName = updates.name ?? columnName;
  const rename = (name: string) => (name === columnName ? newName : name);

  return {
    ...board,
    columns: board.columns.map((c) => {
      if (c.name !== columnName) return c;
      const updated = { ...c, name: newName, color: updates.color ?? c.color };
      if (updates.category !== undefined) {
        if (updates.category === '') {
          delete updated.category;
        } else {
          updated.category = updates.category;
        }
      }
      return updated;
    }),
    default_column: rename(board.default_column),
    ready: board.ready?.order
      ? { ...board.ready, order: board.ready.order.map(rename) }
      : board.ready,
  };
}
