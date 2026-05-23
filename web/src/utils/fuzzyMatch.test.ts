import { describe, it, expect } from 'vitest';
import { cardMatchesQuery, fuzzyMatch } from './fuzzyMatch';
import type { Card, BoardConfig } from '../api/types';

function makeCard(overrides: Partial<Card> = {}): Card {
  return {
    id: 'a_sQiNQyD3',
    alias: 'omnibar-search',
    alias_explicit: true,
    title: 'omnibar search should include card IDs',
    description: 'Searching by ID makes it easy to jump to a known card.',
    column: 'in-progress',
    creator: 'Alexander Terp',
    created_at_millis: 0,
    updated_at_millis: 0,
    ...overrides,
  };
}

const emptyBoard: BoardConfig = {
  id: 'b_test',
  name: 'test',
  columns: [],
  default_column: 'todo',
};

const boardWithCustomFields: BoardConfig = {
  ...emptyBoard,
  custom_fields: {
    type: { type: 'enum' },
    tags: { type: 'enum-set' },
  },
};

describe('fuzzyMatch', () => {
  it('matches case-insensitively', () => {
    expect(fuzzyMatch('BUG', 'fixing a bug')).toBe(true);
    expect(fuzzyMatch('fix', 'PREFIX')).toBe(true);
  });

  it('requires consecutive substring', () => {
    expect(fuzzyMatch('fg', 'fixing a bug')).toBe(false);
  });
});

describe('cardMatchesQuery', () => {
  it('returns true for an empty query', () => {
    expect(cardMatchesQuery(makeCard(), '', emptyBoard)).toBe(true);
    expect(cardMatchesQuery(makeCard(), '   ', emptyBoard)).toBe(true);
  });

  describe('id search', () => {
    it('matches a full card id', () => {
      expect(cardMatchesQuery(makeCard(), 'a_sQiNQyD3', emptyBoard)).toBe(true);
    });

    it('matches a partial id substring', () => {
      expect(cardMatchesQuery(makeCard(), 'sqiNQy', emptyBoard)).toBe(true);
    });

    it('matches the id prefix', () => {
      expect(cardMatchesQuery(makeCard(), 'a_sqi', emptyBoard)).toBe(true);
    });

    it('does not match an unrelated id', () => {
      const card = makeCard({
        id: 'a_XYZ123',
        title: 'unrelated title',
        alias: 'unrelated-alias',
        description: undefined,
      });
      expect(cardMatchesQuery(card, 'sqinqy', emptyBoard)).toBe(false);
    });
  });

  describe('existing field behavior still works', () => {
    it('matches by title', () => {
      expect(cardMatchesQuery(makeCard(), 'include card', emptyBoard)).toBe(true);
    });

    it('matches by alias', () => {
      expect(cardMatchesQuery(makeCard(), 'omnibar-search', emptyBoard)).toBe(true);
    });

    it('matches by description', () => {
      expect(cardMatchesQuery(makeCard(), 'jump to a known', emptyBoard)).toBe(true);
    });

    it('matches by custom field value', () => {
      const card = makeCard({ type: 'feature', tags: ['ui', 'search'] });
      expect(cardMatchesQuery(card, 'feature', boardWithCustomFields)).toBe(true);
      expect(cardMatchesQuery(card, 'search', boardWithCustomFields)).toBe(true);
    });

    it('combines words with AND across fields', () => {
      // "feature" in custom field, "omnibar" in title
      const card = makeCard({ type: 'feature' });
      expect(cardMatchesQuery(card, 'feature omnibar', boardWithCustomFields)).toBe(true);
    });

    it('fails when one word matches nothing', () => {
      expect(cardMatchesQuery(makeCard(), 'omnibar nonexistentword', emptyBoard)).toBe(false);
    });
  });
});
