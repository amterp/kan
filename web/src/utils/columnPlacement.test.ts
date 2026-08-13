import { describe, expect, it } from 'vitest';
import type { BoardConfig } from '../api/types';
import { APPEND_POSITION, moveInsertsAtTop, resolveInsertIndex } from './columnPlacement';

const board: BoardConfig = {
  id: 'b1',
  name: 'main',
  default_column: 'backlog',
  columns: [
    { name: 'backlog', color: '#000', on_move_default_position: 'bottom' },
    { name: 'in-progress', color: '#000', on_move_default_position: 'top' },
    { name: 'done', color: '#000' },
  ],
  custom_fields: {},
} as BoardConfig;

describe('moveInsertsAtTop', () => {
  it('defaults to top when unset', () => {
    expect(moveInsertsAtTop({ name: 'done', color: '#000' })).toBe(true);
  });

  it('respects an explicit setting', () => {
    expect(moveInsertsAtTop({ name: 'a', color: '#000', on_move_default_position: 'top' })).toBe(true);
    expect(moveInsertsAtTop({ name: 'a', color: '#000', on_move_default_position: 'bottom' })).toBe(false);
  });

  it('treats a missing column as top', () => {
    expect(moveInsertsAtTop(undefined)).toBe(true);
  });
});

describe('resolveInsertIndex', () => {
  it('honors an explicit in-range position', () => {
    expect(resolveInsertIndex(board, 'in-progress', 3, 2, true, -1)).toBe(2);
  });

  it('appends for an explicit out-of-range position', () => {
    expect(resolveInsertIndex(board, 'in-progress', 3, -1, true, -1)).toBe(3);
    expect(resolveInsertIndex(board, 'in-progress', 3, 99, true, -1)).toBe(3);
  });

  // Negatives count back from the end, matching computePosition on the server.
  // Only -1 is sent today, but the two implementations claim to mirror each
  // other, and a silent disagreement here renders the card in the wrong slot
  // until the next refresh moves it.
  it('counts negative positions back from the end', () => {
    expect(resolveInsertIndex(board, 'in-progress', 3, -2, true, -1)).toBe(2);
    expect(resolveInsertIndex(board, 'in-progress', 3, -3, true, -1)).toBe(1);
    expect(resolveInsertIndex(board, 'in-progress', 3, -4, true, -1)).toBe(0);
  });

  it('clamps a negative position that underflows past the top', () => {
    expect(resolveInsertIndex(board, 'in-progress', 3, -99, true, -1)).toBe(0);
  });

  // The regression behind v0.29.1: dropping a card below every card in a column
  // must land it at the bottom even though the column places arrivals at the
  // top. APPEND_POSITION is what says so - omitting the position instead hands
  // the choice to on_move_default_position and the card jumps to the top.
  it('sends a bottom drop to the end regardless of the column setting', () => {
    expect(resolveInsertIndex(board, 'in-progress', 3, APPEND_POSITION, true, -1)).toBe(3);
    expect(resolveInsertIndex(board, 'backlog', 3, APPEND_POSITION, true, -1)).toBe(3);
    expect(resolveInsertIndex(board, 'done', 3, APPEND_POSITION, true, -1)).toBe(3);
  });

  it('sends a bottom drop within a column to the end', () => {
    expect(resolveInsertIndex(board, 'in-progress', 3, APPEND_POSITION, false, 0)).toBe(3);
  });

  it('uses the column setting when no position is given', () => {
    expect(resolveInsertIndex(board, 'in-progress', 3, undefined, true, -1)).toBe(0);
    expect(resolveInsertIndex(board, 'backlog', 3, undefined, true, -1)).toBe(3);
  });

  it('defaults an unconfigured column to top', () => {
    expect(resolveInsertIndex(board, 'done', 3, undefined, true, -1)).toBe(0);
  });

  it('leaves a same-column move in place', () => {
    expect(resolveInsertIndex(board, 'in-progress', 3, undefined, false, 2)).toBe(2);
  });
});
