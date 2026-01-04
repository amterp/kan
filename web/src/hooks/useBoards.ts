import { useState, useEffect, useCallback } from 'react';
import { listBoards, getBoard } from '../api/boards';
import { listCards, moveCard as apiMoveCard, createCard as apiCreateCard, updateCard as apiUpdateCard, deleteCard as apiDeleteCard } from '../api/cards';
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

  const moveCard = useCallback(async (cardId: string, newColumn: string, position?: number) => {
    if (!boardName || !board) return;

    // Optimistic update: move card in local state immediately
    setCards((prevCards) => {
      const cardToMove = prevCards.find((c) => c.id === cardId);
      if (!cardToMove) return prevCards;

      // Remove card from its current position
      const withoutCard = prevCards.filter((c) => c.id !== cardId);

      // Build new cards array with proper ordering per column
      const cardsByColumn: Record<string, typeof prevCards> = {};
      for (const col of board.columns) {
        cardsByColumn[col.name] = withoutCard.filter((c) => c.column === col.name);
      }

      // Insert card into target column at position
      const updatedCard = { ...cardToMove, column: newColumn, updated_at_millis: Date.now() };
      const targetColumnCards = cardsByColumn[newColumn] || [];
      if (position !== undefined && position >= 0 && position < targetColumnCards.length) {
        targetColumnCards.splice(position, 0, updatedCard);
      } else {
        targetColumnCards.push(updatedCard);
      }
      cardsByColumn[newColumn] = targetColumnCards;

      // Flatten back to array, maintaining column order
      const result: typeof prevCards = [];
      for (const col of board.columns) {
        result.push(...(cardsByColumn[col.name] || []));
      }
      return result;
    });

    try {
      await apiMoveCard(boardName, cardId, newColumn, position);
      // No refresh needed - optimistic update already applied
    } catch (e) {
      // Revert on error by refreshing from server
      await refresh();
      throw e;
    }
  }, [boardName, board, refresh]);

  const createCard = useCallback(async (input: CreateCardInput) => {
    if (!boardName) return;

    const newCard = await apiCreateCard(boardName, input);
    setCards((prev) => [...prev, newCard]);
    return newCard;
  }, [boardName]);

  const updateCard = useCallback(async (cardId: string, updates: Partial<Card>) => {
    if (!boardName) return;

    // Optimistic update
    setCards((prev) =>
      prev.map((card) =>
        card.id === cardId ? { ...card, ...updates, updated_at_millis: Date.now() } : card
      )
    );

    try {
      const updatedCard = await apiUpdateCard(boardName, cardId, updates);
      // Update with server response
      setCards((prev) =>
        prev.map((card) =>
          card.id === cardId ? updatedCard : card
        )
      );
    } catch (e) {
      // Revert on error
      refresh();
      throw e;
    }
  }, [boardName, refresh]);

  const deleteCard = useCallback(async (cardId: string) => {
    if (!boardName) return;

    // Optimistic update: remove from local state immediately
    setCards((prev) => prev.filter((card) => card.id !== cardId));

    try {
      await apiDeleteCard(boardName, cardId);
    } catch (e) {
      // Revert on error
      refresh();
      throw e;
    }
  }, [boardName, refresh]);

  return { board, cards, loading, error, moveCard, createCard, updateCard, deleteCard, refresh };
}
