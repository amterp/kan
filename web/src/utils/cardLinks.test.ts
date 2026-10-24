import { describe, expect, it } from 'vitest';
import type { Card, Column } from '../api/types';
import { COLUMN_CATEGORIES } from '../api/types';
import { getBlockedCardIds, getCardBlockers, getCardsBlockedBy } from './cardLinks';

const card = (id: string, updates: Partial<Card> = {}): Card => ({
  id, alias: id, alias_explicit: false, title: id, column: 'todo', creator: 'tester',
  created_at_millis: 0, updated_at_millis: 0, ...updates,
});
const columns: Column[] = [{ name: 'todo', color: '', category: 'open' }, { name: 'done', color: '', category: 'done' }];

describe('card links', () => {
  it('leaves cards without blockers unblocked and ignores related links', () => {
    expect(getBlockedCardIds([card('source', { related: ['missing'] }), card('other')], columns).size).toBe(0);
  });

  it.each(COLUMN_CATEGORIES)('only a done blocker releases its waiting card: %s', (category) => {
    const cards = [card('source', { blocked_by: ['target'] }), card('target')];
    const result = getBlockedCardIds(cards, [{ name: 'todo', color: '', category }]);
    expect(result.has('source')).toBe(category !== 'done');
  });

  it('requires every blocker to be done', () => {
    const cards = [card('source', { blocked_by: ['finished', 'unfinished'] }), card('finished', { column: 'done' }), card('unfinished')];
    expect(getBlockedCardIds(cards, columns).has('source')).toBe(true);
  });

  it('keeps missing, qualified, uncategorized, and unknown-column blockers blocked', () => {
    const cards = [
      card('missing', { blocked_by: ['not-loaded'] }),
      card('qualified', { blocked_by: ['board:target'] }),
      card('uncategorized', { blocked_by: ['no-category'] }),
      card('unknown-column', { blocked_by: ['unknown'] }),
      card('no-category', { column: 'custom' }), card('unknown', { column: 'deleted' }),
      card('board:target', { column: 'done' }),
    ];
    const result = getBlockedCardIds(cards, [...columns, { name: 'custom', color: '' }]);
    expect([...result]).toEqual(['missing', 'qualified', 'uncategorized', 'unknown-column']);
  });

  it('recomputes waiting cards when only a blocker changes', () => {
    const source = card('source', { blocked_by: ['target'] });
    const target = card('target');
    expect(getBlockedCardIds([source, target], columns).has(source.id)).toBe(true);
    expect(getBlockedCardIds([source, { ...target, column: 'done' }], columns).has(source.id)).toBe(false);
  });

  it('recomputes waiting cards when a column category changes', () => {
    const cards = [card('source', { blocked_by: ['target'] }), card('target')];
    expect(getBlockedCardIds(cards, [{ name: 'todo', color: '' }]).has('source')).toBe(true);
    expect(getBlockedCardIds(cards, [{ name: 'todo', color: '', category: 'done' }]).has('source')).toBe(false);
  });

  it('lists blockers once and leaves unavailable refs unresolved', () => {
    const target = card('target');
    const source = card('source', { blocked_by: ['target', 'target', 'gone', 'board:target'] });
    expect(getCardBlockers(source, [target, source])).toEqual([
      { ref: 'target', card: target }, { ref: 'gone', card: undefined }, { ref: 'board:target', card: undefined },
    ]);
  });

  it('finds incoming same-board blockers and ignores related and qualified refs', () => {
    const waiting = card('waiting', { blocked_by: ['target'] });
    const cards = [waiting, card('related', { related: ['target'] }), card('cross-board', { blocked_by: ['board:target'] })];
    expect(getCardsBlockedBy('target', cards)).toEqual([waiting]);
  });
});
