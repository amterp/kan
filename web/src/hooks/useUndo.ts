import { useEffect, useRef, useCallback } from 'react';
import type { Card, BoardConfig, UpdateCardInput } from '../api/types';
import type { ToastType } from '../contexts/ToastContext';

const MAX_UNDO_DEPTH = 20;

// Known Card fields that aren't custom fields
const KNOWN_CARD_KEYS = new Set([
  'id', 'alias', 'alias_explicit', 'title', 'description',
  'column', 'parent', 'creator', 'created_at_millis', 'updated_at_millis',
  'comments', 'missing_wanted_fields',
]);

export type UndoAction =
  | {
      type: 'move';
      cardId: string;
      fromColumn: string;
      fromPosition: number;
      toColumn: string;
      toPosition?: number;
    }
  | {
      type: 'delete';
      card: Card;
      column: string;
      position: number;
    }
  | {
      type: 'edit';
      cardId: string;
      fieldChanges: Record<string, { from: unknown; to: unknown }>;
    };

interface UseUndoOptions {
  boardName: string | null;
  cards: Card[];
  board: BoardConfig | null;
  moveCard: (cardId: string, column: string, position?: number) => Promise<void>;
  updateCard: (cardId: string, updates: UpdateCardInput) => Promise<void>;
  deleteCard: (cardId: string) => Promise<void>;
  restoreCard: (board: string, card: Card, column: string, position: number) => Promise<Card>;
  addCardToState: (card: Card) => void;
  showToast: (type: ToastType, message: string) => void;
}

function boardSchemaKey(board: BoardConfig | null): string {
  if (!board) return '';
  return JSON.stringify({
    cf: board.custom_fields,
    cols: board.columns.map((c) => ({ name: c.name, limit: c.limit })),
  });
}

// Build an UpdateCardInput from field changes, applying either 'from' or 'to' values.
// Returns null if no fields can be applied (all stale).
function buildFieldUpdate(
  card: Card,
  fieldChanges: Record<string, { from: unknown; to: unknown }>,
  direction: 'from' | 'to',
  showToast: (type: ToastType, message: string) => void,
): UpdateCardInput | null {
  // The "expected" value is the opposite of what we're applying.
  // For undo (direction='from'), we expect current to match 'to'.
  // For redo (direction='to'), we expect current to match 'from'.
  const expectedKey = direction === 'from' ? 'to' : 'from';

  const updates: UpdateCardInput = {};
  const customFields: Record<string, unknown> = {};
  let appliedCount = 0;
  let staleCount = 0;

  for (const [key, change] of Object.entries(fieldChanges)) {
    const currentValue = card[key];
    const isStale = JSON.stringify(currentValue) !== JSON.stringify(change[expectedKey]);

    if (isStale) {
      staleCount++;
      continue;
    }

    appliedCount++;
    const value = change[direction];
    if (KNOWN_CARD_KEYS.has(key)) {
      if (key === 'title') updates.title = value as string;
      else if (key === 'description') updates.description = value as string;
      else if (key === 'column') updates.column = value as string;
    } else {
      customFields[key] = value;
    }
  }

  if (appliedCount === 0) {
    showToast('info', 'All fields were changed externally, skipped');
    return null;
  }

  if (Object.keys(customFields).length > 0) {
    updates.custom_fields = customFields;
  }

  if (staleCount > 0) {
    const label = direction === 'from' ? 'undone' : 'redone';
    const fieldWord = staleCount === 1 ? 'field was' : 'fields were';
    showToast('info', `Partially ${label} \u2013 ${staleCount} ${fieldWord} changed externally`);
  }

  return updates;
}

export function useUndo(options: UseUndoOptions) {
  const stackRef = useRef<UndoAction[]>([]);
  const redoStackRef = useRef<UndoAction[]>([]);
  const operationInProgressRef = useRef(false);
  const schemaKeyRef = useRef(boardSchemaKey(options.board));

  // Keep a ref to cards so the keyboard handler always sees the latest state
  const cardsRef = useRef(options.cards);
  cardsRef.current = options.cards;

  // Keep refs to callbacks so the keyboard handler closure doesn't go stale
  const optionsRef = useRef(options);
  optionsRef.current = options;

  const clearStacks = useCallback(() => {
    stackRef.current = [];
    redoStackRef.current = [];
  }, []);

  // Clear stacks on board switch
  useEffect(() => {
    clearStacks();
  }, [options.boardName, clearStacks]);

  // Flush stacks on board schema change
  useEffect(() => {
    const newKey = boardSchemaKey(options.board);
    if (schemaKeyRef.current && newKey && newKey !== schemaKeyRef.current) {
      clearStacks();
    }
    schemaKeyRef.current = newKey;
  }, [options.board, clearStacks]);

  const pushUndo = useCallback((action: UndoAction) => {
    const stack = stackRef.current;
    stack.push(action);
    if (stack.length > MAX_UNDO_DEPTH) {
      stack.shift();
    }
    // New action forks the timeline - discard redo history
    redoStackRef.current = [];
  }, []);

  const performUndo = useCallback(async () => {
    if (operationInProgressRef.current) return;

    const action = stackRef.current.pop();
    if (!action) return;

    operationInProgressRef.current = true;
    const opts = optionsRef.current;
    const currentCards = cardsRef.current;
    let succeeded = false;

    try {
      switch (action.type) {
        case 'move': {
          const card = currentCards.find((c) => c.id === action.cardId);
          if (!card) {
            opts.showToast('info', 'Card no longer exists, undo skipped');
            break;
          }
          if (card.column !== action.toColumn) {
            opts.showToast('info', 'Card was moved externally, undo skipped');
            break;
          }
          await opts.moveCard(action.cardId, action.fromColumn, action.fromPosition);
          succeeded = true;
          break;
        }

        case 'delete': {
          if (!opts.boardName) break;
          const restored = await opts.restoreCard(
            opts.boardName,
            action.card,
            action.column,
            action.position,
          );
          opts.addCardToState(restored);
          succeeded = true;
          break;
        }

        case 'edit': {
          const card = currentCards.find((c) => c.id === action.cardId);
          if (!card) {
            opts.showToast('info', 'Card no longer exists, undo skipped');
            break;
          }
          const updates = buildFieldUpdate(card, action.fieldChanges, 'from', opts.showToast);
          if (!updates) break;
          await opts.updateCard(action.cardId, updates);
          succeeded = true;
          break;
        }
      }

      if (succeeded) {
        redoStackRef.current.push(action);
      }
    } catch (e) {
      // Re-push the action so the user can retry
      stackRef.current.push(action);
      const message = e instanceof Error ? e.message : 'Undo failed';
      opts.showToast('error', message);
    } finally {
      operationInProgressRef.current = false;
    }
  }, []);

  const performRedo = useCallback(async () => {
    if (operationInProgressRef.current) return;

    const action = redoStackRef.current.pop();
    if (!action) return;

    operationInProgressRef.current = true;
    const opts = optionsRef.current;
    const currentCards = cardsRef.current;
    let succeeded = false;

    try {
      switch (action.type) {
        case 'move': {
          const card = currentCards.find((c) => c.id === action.cardId);
          if (!card) {
            opts.showToast('info', 'Card no longer exists, redo skipped');
            break;
          }
          if (card.column !== action.fromColumn) {
            opts.showToast('info', 'Card was moved externally, redo skipped');
            break;
          }
          await opts.moveCard(action.cardId, action.toColumn, action.toPosition);
          succeeded = true;
          break;
        }

        case 'delete': {
          const card = currentCards.find((c) => c.id === action.card.id);
          if (!card) {
            opts.showToast('info', 'Card no longer exists, redo skipped');
            break;
          }
          await opts.deleteCard(action.card.id);
          opts.showToast('info', 'Card deleted \u2013 Cmd+Z to undo');
          succeeded = true;
          break;
        }

        case 'edit': {
          const card = currentCards.find((c) => c.id === action.cardId);
          if (!card) {
            opts.showToast('info', 'Card no longer exists, redo skipped');
            break;
          }
          const updates = buildFieldUpdate(card, action.fieldChanges, 'to', opts.showToast);
          if (!updates) break;
          await opts.updateCard(action.cardId, updates);
          succeeded = true;
          break;
        }
      }

      if (succeeded) {
        const stack = stackRef.current;
        stack.push(action);
        if (stack.length > MAX_UNDO_DEPTH) {
          stack.shift();
        }
      }
    } catch (e) {
      // Re-push so the user can retry
      redoStackRef.current.push(action);
      const message = e instanceof Error ? e.message : 'Redo failed';
      opts.showToast('error', message);
    } finally {
      operationInProgressRef.current = false;
    }
  }, []);

  // Global keyboard listener for Cmd+Z (undo) and Cmd+Shift+Z (redo)
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return;
      if (e.key !== 'z' && e.key !== 'Z') return;

      // Don't intercept native undo/redo in form elements
      const el = document.activeElement;
      if (el) {
        const tag = el.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
        if ((el as HTMLElement).isContentEditable) return;
      }

      if (e.shiftKey) {
        if (redoStackRef.current.length === 0) return;
        e.preventDefault();
        performRedo();
      } else {
        if (stackRef.current.length === 0) return;
        e.preventDefault();
        performUndo();
      }
    };

    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [performUndo, performRedo]);

  return { pushUndo, clearStack: clearStacks };
}
