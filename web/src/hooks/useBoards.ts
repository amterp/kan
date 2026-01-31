import { useState, useEffect, useCallback, useRef } from 'react';
import { listBoards, getBoard, createColumn as apiCreateColumn, deleteColumn as apiDeleteColumn, updateColumn as apiUpdateColumn, reorderColumns as apiReorderColumns } from '../api/boards';
import { listCards, moveCard as apiMoveCard, createCard as apiCreateCard, updateCard as apiUpdateCard, deleteCard as apiDeleteCard, getCard as apiGetCard } from '../api/cards';
import type { BoardConfig, Card, CreateCardInput, CreateCardResponse, UpdateCardInput, CreateColumnInput, UpdateColumnInput } from '../api/types';
import { useFileSync, type FileChange } from './useFileSync';

export function useBoards(refreshKey = 0) {
  const [boards, setBoards] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchBoards = useCallback(async () => {
    try {
      const result = await listBoards();
      setBoards(result);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load boards');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchBoards();
  }, [fetchBoards, refreshKey]);

  return { boards, loading, error };
}

export function useBoard(boardName: string | null, refreshKey = 0) {
  const [board, setBoard] = useState<BoardConfig | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Track pending local changes to avoid overwriting optimistic updates
  const pendingChangesRef = useRef<Set<string>>(new Set());

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

  // Handle file change notifications from the server
  const handleCardChange = useCallback(async (change: FileChange) => {
    if (!boardName || change.board_name !== boardName) return;

    // Skip if this card has pending local changes
    if (change.card_id && pendingChangesRef.current.has(change.card_id)) {
      return;
    }

    if (change.type === 'deleted' && change.card_id) {
      // Remove deleted card from state
      setCards((prev) => prev.filter((c) => c.id !== change.card_id));
    } else if (change.type === 'created' || change.type === 'modified') {
      // Fetch the updated card from the server
      if (change.card_id) {
        try {
          const updatedCard = await apiGetCard(boardName, change.card_id);
          setCards((prev) => {
            const existing = prev.find((c) => c.id === change.card_id);
            if (existing) {
              // Update existing card
              return prev.map((c) => (c.id === change.card_id ? updatedCard : c));
            } else {
              // Add new card
              return [...prev, updatedCard];
            }
          });
        } catch (err) {
          // Card might have been deleted between notification and fetch
          console.warn('Failed to fetch updated card:', err);
        }
      }
    }
  }, [boardName]);

  const handleBoardChange = useCallback(async () => {
    // Board config changed - refresh both board AND cards
    // Cards need refresh because their column assignments come from board config
    if (!boardName) return;
    try {
      const [boardData, cardsData] = await Promise.all([
        getBoard(boardName),
        listCards(boardName),
      ]);
      setBoard(boardData);
      // Merge fetched cards, but preserve any with pending local changes
      setCards((prev) => {
        const pendingIds = pendingChangesRef.current;
        if (pendingIds.size === 0) {
          return cardsData;
        }
        // Keep local versions of cards with pending changes
        const pendingCards = prev.filter((c) => pendingIds.has(c.id));
        const freshCards = cardsData.filter((c) => !pendingIds.has(c.id));
        return [...freshCards, ...pendingCards];
      });
    } catch (err) {
      console.warn('Failed to refresh board:', err);
    }
  }, [boardName]);

  // Enable file sync when we have a board
  const { connected: fileSyncConnected, reconnecting: fileSyncReconnecting, failed: fileSyncFailed } = useFileSync({
    onCardChange: handleCardChange,
    onBoardChange: handleBoardChange,
    boardFilter: boardName || undefined,
    enabled: !!boardName,
  });

  useEffect(() => {
    if (boardName) {
      refresh();
    }
  }, [boardName, refresh, refreshKey]);

  const moveCard = useCallback(async (cardId: string, newColumn: string, position?: number) => {
    if (!boardName || !board) return;

    // Mark card as having pending changes to prevent WebSocket overwrites
    pendingChangesRef.current.add(cardId);

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
    } finally {
      pendingChangesRef.current.delete(cardId);
    }
  }, [boardName, board, refresh]);

  const createCard = useCallback(async (input: CreateCardInput): Promise<CreateCardResponse | undefined> => {
    if (!boardName) return;

    const response = await apiCreateCard(boardName, input);
    setCards((prev) => [...prev, response.card]);

    // Log hook results if any ran
    if (response.hook_results && response.hook_results.length > 0) {
      for (const hook of response.hook_results) {
        if (hook.success) {
          if (hook.output) {
            console.log(`[hook: ${hook.name}]`, hook.output);
          }
        } else {
          console.warn(`[hook: ${hook.name}] failed:`, hook.error);
        }
      }
    }

    return response;
  }, [boardName]);

  const updateCard = useCallback(async (cardId: string, updates: UpdateCardInput) => {
    if (!boardName) return;

    // Mark card as having pending changes to prevent WebSocket overwrites
    pendingChangesRef.current.add(cardId);

    // Optimistic update (basic fields only; custom fields come from server response)
    setCards((prev) =>
      prev.map((card) =>
        card.id === cardId ? {
          ...card,
          title: updates.title ?? card.title,
          description: updates.description ?? card.description,
          column: updates.column ?? card.column,
          updated_at_millis: Date.now(),
        } : card
      )
    );

    try {
      const updatedCard = await apiUpdateCard(boardName, cardId, updates);
      // Update with server response (includes custom fields)
      setCards((prev) =>
        prev.map((card) =>
          card.id === cardId ? updatedCard : card
        )
      );
    } catch (e) {
      // Revert on error
      refresh();
      throw e;
    } finally {
      pendingChangesRef.current.delete(cardId);
    }
  }, [boardName, refresh]);

  const deleteCard = useCallback(async (cardId: string) => {
    if (!boardName) return;

    // Mark card as having pending changes to prevent WebSocket overwrites
    pendingChangesRef.current.add(cardId);

    // Optimistic update: remove from local state immediately
    setCards((prev) => prev.filter((card) => card.id !== cardId));

    try {
      await apiDeleteCard(boardName, cardId);
    } catch (e) {
      // Revert on error
      refresh();
      throw e;
    } finally {
      pendingChangesRef.current.delete(cardId);
    }
  }, [boardName, refresh]);

  // Column operations

  const createColumn = useCallback(async (input: CreateColumnInput) => {
    if (!boardName) return;

    const newColumn = await apiCreateColumn(boardName, input);
    // Refresh board to get updated columns list
    await refresh();
    return newColumn;
  }, [boardName, refresh]);

  const deleteColumn = useCallback(async (columnName: string) => {
    if (!boardName || !board) return;

    // Optimistic update: remove column and its cards
    setCards((prev) => prev.filter((c) => c.column !== columnName));
    setBoard((prev) =>
      prev ? { ...prev, columns: prev.columns.filter((c) => c.name !== columnName) } : prev
    );

    try {
      const result = await apiDeleteColumn(boardName, columnName);
      return result.deleted_cards;
    } catch (e) {
      // Revert on error
      refresh();
      throw e;
    }
  }, [boardName, board, refresh]);

  const updateColumn = useCallback(async (columnName: string, updates: UpdateColumnInput) => {
    if (!boardName || !board) return;

    // Optimistic update
    setBoard((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        columns: prev.columns.map((c) =>
          c.name === columnName
            ? { ...c, name: updates.name ?? c.name, color: updates.color ?? c.color }
            : c
        ),
        // Update default_column if it was renamed
        default_column:
          prev.default_column === columnName && updates.name
            ? updates.name
            : prev.default_column,
      };
    });

    // Update card columns if column was renamed
    if (updates.name && updates.name !== columnName) {
      setCards((prev) =>
        prev.map((c) => (c.column === columnName ? { ...c, column: updates.name! } : c))
      );
    }

    try {
      const updated = await apiUpdateColumn(boardName, columnName, updates);
      return updated;
    } catch (e) {
      // Revert on error
      refresh();
      throw e;
    }
  }, [boardName, board, refresh]);

  const reorderColumns = useCallback(async (columnOrder: string[]) => {
    if (!boardName || !board) return;

    // Optimistic update: reorder columns in local state
    setBoard((prev) => {
      if (!prev) return prev;
      const columnMap = new Map(prev.columns.map((c) => [c.name, c]));
      const reordered = columnOrder.map((name) => columnMap.get(name)!).filter(Boolean);
      return { ...prev, columns: reordered };
    });

    try {
      await apiReorderColumns(boardName, columnOrder);
    } catch (e) {
      // Revert on error
      refresh();
      throw e;
    }
  }, [boardName, board, refresh]);

  return {
    board,
    cards,
    loading,
    error,
    moveCard,
    createCard,
    updateCard,
    deleteCard,
    createColumn,
    deleteColumn,
    updateColumn,
    reorderColumns,
    refresh,
    fileSyncConnected,
    fileSyncReconnecting,
    fileSyncFailed,
  };
}
