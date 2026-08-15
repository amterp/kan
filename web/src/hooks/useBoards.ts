import { useState, useEffect, useCallback, useRef } from 'react';
import { listBoards, getBoard, createColumn as apiCreateColumn, deleteColumn as apiDeleteColumn, updateColumn as apiUpdateColumn, reorderColumns as apiReorderColumns } from '../api/boards';
import { listCards, moveCard as apiMoveCard, createCard as apiCreateCard, updateCard as apiUpdateCard, deleteCard as apiDeleteCard, getCard as apiGetCard } from '../api/cards';
import { ApiError } from '../api/client';
import type { BoardConfig, Card, CreateCardInput, CreateCardResponse, FileChange, HookInfo, UpdateCardInput, CreateColumnInput, UpdateColumnInput } from '../api/types';
import { useFileSyncStatus, useFileSyncSubscription } from '../contexts/FileSyncContext';
import { useToast } from '../contexts/ToastContext';
import { resolveInsertIndex } from '../utils/columnPlacement';
import { upsertCard } from '../utils/cardSync';

// A failed hook still creates the card, so the board looks entirely normal. Without a
// visible message the only symptom is the hook's effect not happening, which reads as
// "my pattern didn't match" rather than "my hook could not run".
function hookFailureMessage(hook: HookInfo): string {
  // A negative exit code is an internal "no exit code" sentinel, not a real status.
  const exit = hook.exit_code && hook.exit_code > 0 ? ` (exit ${hook.exit_code})` : '';
  const detail = hook.stderr || hook.error;

  let message = `Hook '${hook.name}' failed${exit}`;
  if (detail) message += `: ${detail}`;
  if (hook.hint) message += `. ${hook.hint}`;
  return message;
}

export function useBoards(refreshKey = 0) {
  const [boards, setBoards] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Returns the fresh list as well as storing it, so a caller that has to act on
  // it immediately - picking where to send the user after the open board vanished
  // - doesn't have to wait a render for the state to land.
  const fetchBoards = useCallback(async (): Promise<string[] | undefined> => {
    try {
      const result = await listBoards();
      setBoards(result);
      setError(null);
      return result;
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load boards');
      return undefined;
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchBoards();
  }, [fetchBoards, refreshKey]);

  // Any board appearing or disappearing on disk changes this list, including
  // boards other than the open one - so this deliberately ignores board_name.
  useFileSyncSubscription(useCallback((change: FileChange) => {
    if (change.kind === 'board') fetchBoards();
  }, [fetchBoards]));

  return { boards, loading, error, refresh: fetchBoards };
}

/**
 * onBoardGone fires when the open board turns out to no longer exist on disk -
 * deleted or renamed from the CLI, or by another tab. The hook cannot decide where
 * to send the user, so the caller handles it.
 */
export function useBoard(boardName: string | null, refreshKey = 0, onBoardGone?: () => void) {
  const [board, setBoard] = useState<BoardConfig | null>(null);
  const [cards, setCards] = useState<Card[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { showToast } = useToast();

  const onBoardGoneRef = useRef(onBoardGone);
  useEffect(() => {
    onBoardGoneRef.current = onBoardGone;
  }, [onBoardGone]);

  // Track pending local changes to avoid overwriting optimistic updates
  const pendingChangesRef = useRef<Set<string>>(new Set());

  // Version counter to discard stale fetch responses after board/project switches
  const fetchVersionRef = useRef(0);

  // Reset state when switching boards so stale data doesn't render
  useEffect(() => {
    fetchVersionRef.current += 1;
    setBoard(null);
    setCards([]);
    setError(null);
    setLoading(!!boardName);
    pendingChangesRef.current = new Set();
  }, [boardName]);

  const refresh = useCallback(async () => {
    if (!boardName) return;
    const version = ++fetchVersionRef.current;
    setLoading(true);
    try {
      const [boardData, cardsData] = await Promise.all([
        getBoard(boardName),
        listCards(boardName),
      ]);
      if (fetchVersionRef.current !== version) return;
      setBoard(boardData);
      setCards(cardsData);
      setError(null);
    } catch (e) {
      if (fetchVersionRef.current !== version) return;
      setError(e instanceof Error ? e.message : 'Failed to load board');
    } finally {
      if (fetchVersionRef.current === version) {
        setLoading(false);
      }
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
          setCards((prev) => upsertCard(prev, updatedCard));
        } catch (err) {
          // Card might have been deleted between notification and fetch
          console.warn('Failed to fetch updated card:', err);
        }
      }
    }
  }, [boardName]);

  const handleBoardChange = useCallback(async (change: FileChange) => {
    // Board config changed - refresh both board AND cards
    // Cards need refresh because their column assignments come from board config
    if (!boardName || change.board_name !== boardName) return;
    const version = ++fetchVersionRef.current;
    try {
      const [boardData, cardsData] = await Promise.all([
        getBoard(boardName),
        listCards(boardName),
      ]);
      if (fetchVersionRef.current !== version) return;
      setBoard(boardData);
      setError(null);
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
      if (fetchVersionRef.current !== version) return;
      // A 404 means the board is gone, not that the refresh failed. Leaving the
      // deleted board on screen would let the user keep editing cards that no
      // longer have a home, so hand it to the caller to navigate away.
      if (err instanceof ApiError && err.status === 404) {
        onBoardGoneRef.current?.();
        return;
      }
      console.warn('Failed to refresh board:', err);
    }
  }, [boardName]);

  // One subscription for the shared connection; each handler filters for itself.
  const { connected: fileSyncConnected, reconnecting: fileSyncReconnecting, failed: fileSyncFailed } = useFileSyncStatus();

  useFileSyncSubscription(useCallback((change: FileChange) => {
    if (change.kind === 'card') handleCardChange(change);
    else if (change.kind === 'board') handleBoardChange(change);
  }, [handleCardChange, handleBoardChange]));

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

      // Insert card into target column, mirroring the server's placement rule so
      // the optimistic render matches what the next refresh will show.
      const updatedCard = { ...cardToMove, column: newColumn, updated_at_millis: Date.now() };
      const targetColumnCards = cardsByColumn[newColumn] || [];
      const currentIndex = prevCards
        .filter((c) => c.column === newColumn)
        .findIndex((c) => c.id === cardId);
      const insertAt = resolveInsertIndex(
        board,
        newColumn,
        targetColumnCards.length,
        position,
        cardToMove.column !== newColumn,
        currentIndex,
      );
      targetColumnCards.splice(insertAt, 0, updatedCard);
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
    // upsertCard covers the case where the WebSocket handler already added this
    // card, as well as the ordinary insert.
    setCards((prev) => upsertCard(prev, response.card));

    // Report hook results. Successes stay quiet in the console; failures get a toast,
    // since a hook that never ran is otherwise indistinguishable from one that didn't match.
    if (response.hook_results && response.hook_results.length > 0) {
      for (const hook of response.hook_results) {
        if (hook.success) {
          if (hook.output) {
            console.log(`[hook: ${hook.name}]`, hook.output);
          }
        } else {
          console.warn(`[hook: ${hook.name}] failed:`, hook.error);
          showToast('error', hookFailureMessage(hook));
        }
      }
    }

    return response;
  }, [boardName, showToast]);

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
      // Update with the server response, which carries custom fields and the
      // authoritative position: changing a card's column is also a move, and the
      // server placed it per that column's on_move_default_position. When nothing
      // moved, the card re-inserts exactly where it already was.
      setCards((prev) => upsertCard(prev, updatedCard));
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

  const addCardToState = useCallback((card: Card) => {
    pendingChangesRef.current.add(card.id);
    setCards((prev) => upsertCard(prev, card));
    // Clear pending after a short delay to let WebSocket settle
    setTimeout(() => pendingChangesRef.current.delete(card.id), 2000);
  }, []);

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
    addCardToState,
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
