import type { Card, Column, BoardLinks, CardLink } from '../api/types';

export function getBlockedCardIds(cards: Card[], columns: Column[], links?: BoardLinks, boardId?: string): Set<string> {
  const byId = new Map(cards.map((card) => [card.id, card]));
  const categories = new Map(columns.map((column) => [column.name, column.category]));
  return new Set(cards.filter((card) => !['done', 'dropped'].includes(categories.get(card.column) ?? '') && card.blocked_by?.some((ref) => {
    if (ref.includes(':')) {
      const target = links?.resolved[ref];
      if (target?.result !== 'found') return true;
      if (boardId && ref.startsWith(`${boardId}:`)) {
        const local = byId.get(ref.slice(boardId.length + 1));
        return !local || categories.get(local.column) !== 'done';
      }
      return target.category !== 'done';
    }
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

export function getCardsBlockedBy(cardId: string, cards: Card[], boardId?: string, links?: BoardLinks): Card[] {
  return cards.filter((card) => card.blocked_by?.some((ref) => ref === cardId || !!boardId && ref === `${boardId}:${cardId}` && links?.resolved[ref]?.result === 'found'));
}

export function resolveCardLinks(refs: string[], cards: Card[], links?: BoardLinks, boardId?: string): CardLink[] {
  const byId = new Map(cards.map((card) => [card.id, card]));
  return [...new Set(refs)].map((ref) => {
    if (ref.includes(':') && links?.resolved[ref]?.result !== 'found') {
      return links?.resolved[ref] ?? { ref, result: 'unreadable' };
    }
    const localId = boardId && ref.startsWith(`${boardId}:`) ? ref.slice(boardId.length + 1) : ref;
    if (!localId.includes(':')) {
      const card = byId.get(localId);
      return { ref: localId, result: card ? 'found' : 'absent', card };
    }
    return links?.resolved[ref] ?? { ref, result: 'unreadable' };
  });
}

export function incomingCardLinks(cardId: string, cards: Card[], links?: BoardLinks, boardId?: string): CardLink[] {
  const local: CardLink[] = getCardsBlockedBy(cardId, cards, boardId, links).map((card) => ({ ref: card.id, result: 'found', card }));
  const found = new Map(local.map((link) => [`${boardId}:${link.card!.id}`, link]));
  for (const link of links?.blocks[cardId] ?? []) {
    if (link.board_id === boardId) continue;
    found.set(`${link.project_path}:${link.board_id}:${link.card?.id ?? link.ref}`, link);
  }
  return [...found.values()];
}
