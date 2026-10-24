import type { Card, Column } from '../api/types';

export function getBlockedCardIds(cards: Card[], columns: Column[]): Set<string> {
  const byId = new Map(cards.map((card) => [card.id, card]));
  const categories = new Map(columns.map((column) => [column.name, column.category]));
  return new Set(cards.filter((card) => !['done', 'dropped'].includes(categories.get(card.column) ?? '') && card.blocked_by?.some((ref) => {
    if (ref.includes(':')) return true;
    const blocker = byId.get(ref);
    return !blocker || categories.get(blocker.column) !== 'done';
  })).map((card) => card.id));
}

export function getCardBlockers(card: Card, cards: Card[]): { ref: string; card?: Card }[] {
  const byId = new Map(cards.map((candidate) => [candidate.id, candidate]));
  return [...new Set(card.blocked_by ?? [])].map((ref) => ({
    ref,
    card: ref.includes(':') ? undefined : byId.get(ref),
  }));
}

export function getCardsBlockedBy(cardId: string, cards: Card[]): Card[] {
  return cards.filter((card) => card.blocked_by?.includes(cardId));
}
