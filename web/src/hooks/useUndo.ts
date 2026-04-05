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

export function useUndo(options: UseUndoOptions) {
  const stackRef = useRef<UndoAction[]>([]);
  const undoInProgressRef = useRef(false);
  const schemaKeyRef = useRef(boardSchemaKey(options.board));

  // Keep a ref to cards so the keyboard handler always sees the latest state
  const cardsRef = useRef(options.cards);
  cardsRef.current = options.cards;

  // Keep refs to callbacks so the keyboard handler closure doesn't go stale
  const optionsRef = useRef(options);
  optionsRef.current = options;

  const clearStack = useCallback(() => {
    stackRef.current = [];
  }, []);

  // Clear stack on board switch
  useEffect(() => {
    clearStack();
  }, [options.boardName, clearStack]);

  // Flush stack on board schema change
  useEffect(() => {
    const newKey = boardSchemaKey(options.board);
    if (schemaKeyRef.current && newKey && newKey !== schemaKeyRef.current) {
      clearStack();
    }
    schemaKeyRef.current = newKey;
  }, [options.board, clearStack]);

  const pushUndo = useCallback((action: UndoAction) => {
    const stack = stackRef.current;
    stack.push(action);
    if (stack.length > MAX_UNDO_DEPTH) {
      stack.shift();
    }
  }, []);

  const performUndo = useCallback(async () => {
    if (undoInProgressRef.current) return;

    const action = stackRef.current.pop();
    if (!action) return;

    undoInProgressRef.current = true;
    const opts = optionsRef.current;
    const currentCards = cardsRef.current;

    try {
      switch (action.type) {
        case 'move': {
          // Staleness check: is the card still in the column we moved it to?
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
          break;
        }

        case 'edit': {
          const card = currentCards.find((c) => c.id === action.cardId);
          if (!card) {
            opts.showToast('info', 'Card no longer exists, undo skipped');
            break;
          }

          // Check each field for staleness and build the revert update
          const updates: UpdateCardInput = {};
          const customFields: Record<string, unknown> = {};
          let revertedCount = 0;
          let staleCount = 0;

          for (const [key, change] of Object.entries(action.fieldChanges)) {
            const currentValue = card[key];
            const isStale = JSON.stringify(currentValue) !== JSON.stringify(change.to);

            if (isStale) {
              staleCount++;
              continue;
            }

            revertedCount++;
            if (KNOWN_CARD_KEYS.has(key)) {
              if (key === 'title') updates.title = change.from as string;
              else if (key === 'description') updates.description = change.from as string;
              else if (key === 'column') updates.column = change.from as string;
            } else {
              customFields[key] = change.from;
            }
          }

          if (revertedCount === 0) {
            opts.showToast('info', 'All fields were changed externally, undo skipped');
            break;
          }

          if (Object.keys(customFields).length > 0) {
            updates.custom_fields = customFields;
          }

          if (staleCount > 0) {
            const fieldWord = staleCount === 1 ? 'field was' : 'fields were';
            opts.showToast('info', `Partially undone \u2013 ${staleCount} ${fieldWord} changed externally`);
          }

          await opts.updateCard(action.cardId, updates);
          break;
        }
      }
    } catch (e) {
      // Re-push the action so the user can retry
      stackRef.current.push(action);
      const message = e instanceof Error ? e.message : 'Undo failed';
      opts.showToast('error', message);
    } finally {
      undoInProgressRef.current = false;
    }
  }, []);

  // Global keyboard listener for Cmd+Z / Ctrl+Z
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (!((e.metaKey || e.ctrlKey) && e.key === 'z' && !e.shiftKey)) return;

      // Don't intercept native undo in form elements
      const el = document.activeElement;
      if (el) {
        const tag = el.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
        if ((el as HTMLElement).isContentEditable) return;
      }

      if (stackRef.current.length === 0) return;

      e.preventDefault();
      performUndo();
    };

    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [performUndo]);

  return { pushUndo, clearStack };
}
