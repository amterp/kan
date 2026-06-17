import { describe, expect, it } from 'vitest';
import type { BoardConfig, Card } from '../api/types';
import { groupCardsByColumn, resolveSortField, sortCards, sortableFieldNames } from './cardSort';

const board: BoardConfig = {
  id: 'b1',
  name: 'main',
  default_column: 'todo',
  columns: [{ name: 'todo', color: '#000' }],
  custom_fields: {
    priority: {
      type: 'enum',
      options: [
        { value: 'ultra-low' },
        { value: 'low' },
        { value: 'medium' },
        { value: 'high' },
      ],
    },
    labels: {
      type: 'enum-set',
      options: [{ value: 'blocked' }, { value: 'needs-review' }],
    },
    assignee: { type: 'string' },
    due: { type: 'date' },
    blocked: { type: 'boolean' },
    topics: { type: 'free-set' },
  },
};

// mkCard builds a minimal card with a position and arbitrary custom fields.
function mkCard(id: string, position: string, fields: Record<string, unknown>): Card {
  return {
    id,
    alias: id,
    alias_explicit: false,
    title: id,
    column: 'todo',
    position,
    creator: 'test',
    created_at_millis: 0,
    updated_at_millis: 0,
    ...fields,
  };
}

const ids = (cards: Card[]) => cards.map((c) => c.id);

describe('sortCards', () => {
  it('orders enum fields by config option order (ascending)', () => {
    // Manual order is descending priority, like the sample board.
    const cards = [
      mkCard('high', 'A', { priority: 'high' }),
      mkCard('medium', 'B', { priority: 'medium' }),
      mkCard('low', 'C', { priority: 'low' }),
      mkCard('ultra', 'D', { priority: 'ultra-low' }),
    ];
    expect(ids(sortCards(cards, board, 'priority', false))).toEqual([
      'ultra',
      'low',
      'medium',
      'high',
    ]);
  });

  it('reverses enum order when descending', () => {
    const cards = [
      mkCard('ultra', 'A', { priority: 'ultra-low' }),
      mkCard('high', 'B', { priority: 'high' }),
      mkCard('low', 'C', { priority: 'low' }),
    ];
    expect(ids(sortCards(cards, board, 'priority', true))).toEqual(['high', 'low', 'ultra']);
  });

  it('places unset cards last in both directions', () => {
    const build = () => [
      mkCard('none1', 'A', {}),
      mkCard('high', 'B', { priority: 'high' }),
      mkCard('none2', 'C', { priority: '' }), // empty string == unset
      mkCard('low', 'D', { priority: 'low' }),
    ];
    expect(ids(sortCards(build(), board, 'priority', false))).toEqual([
      'low',
      'high',
      'none1',
      'none2',
    ]);
    expect(ids(sortCards(build(), board, 'priority', true))).toEqual([
      'high',
      'low',
      'none1',
      'none2',
    ]);
  });

  it('keeps equal-valued cards in manual (position) order regardless of direction', () => {
    const build = () => [
      mkCard('h_late', 'B', { priority: 'high' }),
      mkCard('h_early', 'A', { priority: 'high' }),
      mkCard('low', 'C', { priority: 'low' }),
    ];
    expect(ids(sortCards(build(), board, 'priority', false))).toEqual(['low', 'h_early', 'h_late']);
    expect(ids(sortCards(build(), board, 'priority', true))).toEqual(['h_early', 'h_late', 'low']);
  });

  it('ranks enum-set by lowest option index, empty set last', () => {
    const cards = [
      mkCard('review', 'A', { labels: ['needs-review'] }),
      mkCard('blocked', 'B', { labels: ['blocked', 'needs-review'] }),
      mkCard('empty', 'C', { labels: [] }),
    ];
    expect(ids(sortCards(cards, board, 'labels', false))).toEqual(['blocked', 'review', 'empty']);
  });

  it('orders booleans false before true, unset last', () => {
    const cards = [
      mkCard('t', 'A', { blocked: true }),
      mkCard('f', 'B', { blocked: false }),
      mkCard('u', 'C', {}),
    ];
    expect(ids(sortCards(cards, board, 'blocked', false))).toEqual(['f', 't', 'u']);
  });

  it('sorts strings case-insensitively', () => {
    const cards = [
      mkCard('bob', 'A', { assignee: 'bob' }),
      mkCard('alice', 'B', { assignee: 'Alice' }),
      mkCard('carol', 'C', { assignee: 'carol' }),
    ];
    expect(ids(sortCards(cards, board, 'assignee', false))).toEqual(['alice', 'bob', 'carol']);
  });

  it('sorts ISO dates chronologically', () => {
    const cards = [
      mkCard('mar', 'A', { due: '2026-03-01' }),
      mkCard('jan', 'B', { due: '2026-01-15' }),
      mkCard('feb', 'C', { due: '2026-02-20' }),
    ];
    expect(ids(sortCards(cards, board, 'due', false))).toEqual(['jan', 'feb', 'mar']);
  });

  it('ranks unknown enum values after known options', () => {
    const cards = [
      mkCard('weird', 'A', { priority: 'weird' }),
      mkCard('high', 'B', { priority: 'high' }),
      mkCard('ultra', 'C', { priority: 'ultra-low' }),
    ];
    expect(ids(sortCards(cards, board, 'priority', false))).toEqual(['ultra', 'high', 'weird']);
  });

  it('ranks free-set by alphabetically-first member', () => {
    const cards = [
      mkCard('mango', 'A', { topics: ['mango'] }),
      mkCard('apple', 'B', { topics: ['zebra', 'apple'] }),
    ];
    expect(ids(sortCards(cards, board, 'topics', false))).toEqual(['apple', 'mango']);
  });

  it('returns input order for an empty field', () => {
    const cards = [mkCard('c', 'C', {}), mkCard('a', 'A', {}), mkCard('b', 'B', {})];
    expect(ids(sortCards(cards, board, '', false))).toEqual(['c', 'a', 'b']);
  });

  it('does not mutate the input array', () => {
    const cards = [
      mkCard('high', 'A', { priority: 'high' }),
      mkCard('low', 'B', { priority: 'low' }),
    ];
    const before = ids(cards);
    sortCards(cards, board, 'priority', false);
    expect(ids(cards)).toEqual(before);
  });
});

describe('sortableFieldNames', () => {
  it('returns all custom field names', () => {
    expect(sortableFieldNames(board).sort()).toEqual(
      ['assignee', 'blocked', 'due', 'labels', 'priority', 'topics'].sort()
    );
  });

  it('returns empty array when no custom fields', () => {
    const bare: BoardConfig = { ...board, custom_fields: undefined };
    expect(sortableFieldNames(bare)).toEqual([]);
  });
});

describe('resolveSortField', () => {
  it('returns the field when the board defines it', () => {
    expect(resolveSortField(board, 'priority')).toBe('priority');
  });

  it('returns empty string for an unknown/stale field', () => {
    expect(resolveSortField(board, 'nonexistent')).toBe('');
  });

  it('returns empty string when no field is requested', () => {
    expect(resolveSortField(board, '')).toBe('');
  });
});

describe('groupCardsByColumn', () => {
  const multiCol: BoardConfig = {
    ...board,
    columns: [
      { name: 'todo', color: '#000' },
      { name: 'doing', color: '#111' },
      { name: 'done', color: '#222' },
    ],
  };

  it('groups cards by column and sorts each by the field', () => {
    const cards = [
      mkCard('t-low', 'A', { column: 'todo', priority: 'low' }),
      mkCard('t-high', 'B', { column: 'todo', priority: 'high' }),
      mkCard('d-med', 'C', { column: 'doing', priority: 'medium' }),
    ];
    const groups = groupCardsByColumn(cards, multiCol, 'priority', false);
    expect(ids(groups.todo)).toEqual(['t-low', 't-high']); // ascending option order
    expect(ids(groups.doing)).toEqual(['d-med']);
    expect(groups.done).toEqual([]); // empty column still present
  });

  it('preserves incoming (manual) order when the field is empty or stale', () => {
    const cards = [
      mkCard('b', 'B', { column: 'todo', priority: 'low' }),
      mkCard('a', 'A', { column: 'todo', priority: 'high' }),
    ];
    expect(ids(groupCardsByColumn(cards, multiCol, '', false).todo)).toEqual(['b', 'a']);
    expect(ids(groupCardsByColumn(cards, multiCol, 'nope', false).todo)).toEqual(['b', 'a']);
  });

  it('returns every column key, including empty ones', () => {
    expect(Object.keys(groupCardsByColumn([], multiCol, 'priority', false)).sort()).toEqual([
      'doing',
      'done',
      'todo',
    ]);
  });
});
