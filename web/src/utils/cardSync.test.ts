import { describe, expect, it } from 'vitest';
import type { Card } from '../api/types';
import { upsertCard } from './cardSync';

function card(id: string, position: string, column = 'backlog'): Card {
  return { id, title: id, column, position } as Card;
}

/** Ordering is what this module is about, so assert on ids rather than objects. */
function ids(cards: Card[]): string[] {
  return cards.map((c) => c.id);
}

describe('upsertCard', () => {
  it('re-ranks a card the server moved within its column', () => {
    const cards = [card('a', 'U'), card('b', 'UU'), card('c', 'UUU')];

    // What `kan move c --before a` produces: only c's file changes.
    expect(ids(upsertCard(cards, card('c', 'E')))).toEqual(['c', 'a', 'b']);
  });

  it('re-ranks a card moved to the bottom of its column', () => {
    const cards = [card('a', 'U'), card('b', 'UU'), card('c', 'UUU')];

    expect(ids(upsertCard(cards, card('a', 'UUUU')))).toEqual(['b', 'c', 'a']);
  });

  it('ranks a card moved to another column against that column, not its old one', () => {
    const cards = [
      card('a', 'U'),
      card('x', 'U', 'next'),
      card('y', 'UUU', 'next'),
    ];

    // 'a' lands between x and y, not at its old index.
    expect(ids(upsertCard(cards, card('a', 'UU', 'next')))).toEqual(['x', 'a', 'y']);
  });

  it('breaks position ties by id, matching the server', () => {
    const cards = [card('b', 'U'), card('d', 'U')];

    expect(ids(upsertCard(cards, card('c', 'U')))).toEqual(['b', 'c', 'd']);
    expect(ids(upsertCard(cards, card('a', 'U')))).toEqual(['a', 'b', 'd']);
    expect(ids(upsertCard(cards, card('e', 'U')))).toEqual(['b', 'd', 'e']);
  });

  it('inserts a card it has never seen at its sorted rank', () => {
    const cards = [card('a', 'U'), card('c', 'UUU')];

    expect(ids(upsertCard(cards, card('b', 'UU')))).toEqual(['a', 'b', 'c']);
  });

  it('appends a card that outranks every sibling', () => {
    const cards = [card('a', 'U'), card('b', 'UU')];

    expect(ids(upsertCard(cards, card('c', 'UUU')))).toEqual(['a', 'b', 'c']);
  });

  it('inserts into an empty board and an empty column', () => {
    expect(ids(upsertCard([], card('a', 'U')))).toEqual(['a']);
    expect(ids(upsertCard([card('a', 'U')], card('x', 'U', 'next')))).toEqual(['a', 'x']);
  });

  it('leaves order untouched when nothing about the position changed', () => {
    const cards = [card('a', 'U'), card('b', 'UU'), card('c', 'UUU')];
    const updated = { ...card('b', 'UU'), title: 'renamed' };

    const result = upsertCard(cards, updated);
    expect(ids(result)).toEqual(['a', 'b', 'c']);
    expect(result[1].title).toBe('renamed');
  });

  it('never duplicates a card it already holds', () => {
    const cards = [card('a', 'U'), card('b', 'UU')];

    expect(ids(upsertCard(cards, card('a', 'UUU')))).toEqual(['b', 'a']);
  });

  it('treats a missing position as ranking first', () => {
    const cards = [card('a', 'U'), card('b', 'UU')];
    const positionless = { id: 'z', title: 'z', column: 'backlog' } as Card;

    expect(ids(upsertCard(cards, positionless))).toEqual(['z', 'a', 'b']);
  });
});
