import { useState, useEffect, useCallback } from 'react';
import { listBoards, getBoard } from '../api/boards';
import { listCards, moveCard as apiMoveCard, createCard as apiCreateCard } from '../api/cards';
import type { BoardConfig, Card, CreateCardInput } from '../api/types';

export function useBoards() {
  const [boards, setBoards] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listBoards()
      .then(setBoards)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  return { boards, loading, error };
}

export function useBoard(boardName: string | null) {
  const [board, setBoard] = useState<BoardConfig | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!boardName) return;
    setLoading(true);
    try {
      const [boardData, cardsData] = await Promise.all([
        getBoard(boardName),
        listCards(boardName),
      ]);
      setBoard(boardData);
      setCards(cardsData);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load board');
    } finally {
      setLoading(false);
    }
  }, [boardName]);

  useEffect(() => {
    if (boardName) {
      refresh();
    }
  }, [boardName, refresh]);

  const moveCard = useCallback(async (cardId: string, newColumn: string) => {
    if (!boardName) return;

    // Optimistic update
    setCards((prev) =>
      prev.map((card) =>
        card.id === cardId ? { ...card, column: newColumn } : card
      )
    );

    try {
      await apiMoveCard(boardName, cardId, newColumn);
    } catch (e) {
      // Revert on error
      refresh();
      throw e;
    }
  }, [boardName, refresh]);

  const createCard = useCallback(async (input: CreateCardInput) => {
    if (!boardName) return;

    const newCard = await apiCreateCard(boardName, input);
    setCards((prev) => [...prev, newCard]);
    return newCard;
  }, [boardName]);

  return { board, cards, loading, error, moveCard, createCard, refresh };
}
