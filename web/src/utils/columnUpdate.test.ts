import { describe, expect, it } from 'vitest';
import type { BoardConfig } from '../api/types';
import { applyColumnUpdate } from './columnUpdate';

const board: BoardConfig = {
  id: 'b1',
  name: 'main',
  default_column: 'backlog',
  columns: [
    { name: 'backlog', color: '#111', category: 'open' },
    { name: 'next', color: '#222' },
  ],
  ready: { order: ['next', 'backlog'], exclude: ['labels=blocked'] },
};

describe('applyColumnUpdate', () => {
  it('sets a category', () => {
    const next = applyColumnUpdate(board, 'next', { category: 'open' });
    expect(next.columns[1]).toEqual({ name: 'next', color: '#222', category: 'open' });
  });

  it('clears a category with an empty string', () => {
    const next = applyColumnUpdate(board, 'backlog', { category: '' });
    expect(next.columns[0]).toEqual({ name: 'backlog', color: '#111' });
  });

  it('keeps the category when the update does not mention it', () => {
    const next = applyColumnUpdate(board, 'backlog', { color: '#999' });
    expect(next.columns[0]).toEqual({ name: 'backlog', color: '#999', category: 'open' });
  });

  it('renames the column in default_column and ready.order', () => {
    const next = applyColumnUpdate(board, 'backlog', { name: 'todo' });
    expect(next.columns[0].name).toBe('todo');
    expect(next.default_column).toBe('todo');
    expect(next.ready).toEqual({ order: ['next', 'todo'], exclude: ['labels=blocked'] });
  });

  it('leaves a board without ready settings without them', () => {
    const plain = { ...board, ready: undefined };
    expect(applyColumnUpdate(plain, 'next', { name: 'today' }).ready).toBeUndefined();
  });
});
