import { useMemo, useRef, useEffect } from 'react';
import { useDroppable } from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import type { Card, Column as ColumnType, BoardConfig } from '../api/types';
import CardComponent from './Card';

interface ColumnProps {
  column: ColumnType;
  cards: Card[];
  board: BoardConfig;
  highlightedCardId?: string | null;
  isAddingCard: boolean;
  draftTitle: string;
  onDraftChange: (title: string) => void;
  onStartAddCard: () => void;
  onCancelAddCard: () => void;
  onAddCard: (title: string, openModal: boolean, keepFormOpen?: boolean) => void;
  onCardClick: (card: Card) => void;
  onDeleteCard: (cardId: string) => void;
  activeCard: Card | null;
  isOverColumn: boolean;
  overIndex: number | null;
}

export default function Column({
  column,
  cards,
  board,
  highlightedCardId,
  isAddingCard,
  draftTitle,
  onDraftChange,
  onStartAddCard,
  onCancelAddCard,
  onAddCard,
  onCardClick,
  onDeleteCard,
  activeCard,
  isOverColumn,
  overIndex,
}: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: column.name });
  const inputRef = useRef<HTMLInputElement>(null);
  const formRef = useRef<HTMLFormElement>(null);

  // SortableContext items - just the cards in this column
  // Don't add activeCard for cross-column drags (we use a manual placeholder instead)
  const sortableItems = useMemo(() => {
    return cards.map((c) => c.id);
  }, [cards]);

  // Focus input when entering add mode
  useEffect(() => {
    if (isAddingCard && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isAddingCard]);

  // Click outside to close (but preserve draft)
  useEffect(() => {
    if (!isAddingCard) return;

    const handleClickOutside = (e: MouseEvent) => {
      if (formRef.current && !formRef.current.contains(e.target as Node)) {
        onCancelAddCard();
      }
    };

    // Delay adding listener to avoid immediate trigger from the click that opened the form
    const timeoutId = setTimeout(() => {
      document.addEventListener('mousedown', handleClickOutside);
    }, 0);

    return () => {
      clearTimeout(timeoutId);
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [isAddingCard, onCancelAddCard]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (draftTitle.trim()) {
      // keepFormOpen=true so user can add another card immediately
      onAddCard(draftTitle.trim(), false, true);
      onDraftChange('');
      // Re-focus the input for the next card
      setTimeout(() => inputRef.current?.focus(), 0);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      onDraftChange('');
      onCancelAddCard();
    } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      // Cmd+Enter or Ctrl+Enter - create and open modal
      e.preventDefault();
      if (draftTitle.trim()) {
        onAddCard(draftTitle.trim(), true);
        onDraftChange('');
      }
    }
  };

  return (
    <div
      ref={setNodeRef}
      className={`flex-1 min-w-64 max-w-sm flex flex-col bg-gray-200 dark:bg-gray-800 rounded-lg max-h-full ${
        isOver ? 'ring-2 ring-blue-400' : ''
      }`}
    >
      {/* Column Header */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-gray-300 dark:border-gray-600">
        <button
          onClick={onStartAddCard}
          className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 p-1 rounded hover:bg-gray-300 dark:hover:bg-gray-600"
          title="Add card"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
        <div
          className="w-3 h-3 rounded-full"
          style={{ backgroundColor: column.color }}
        />
        <h2 className="font-medium text-gray-700 dark:text-gray-200">{column.name}</h2>
        <span className="text-sm text-gray-500 dark:text-gray-400">{cards.length}</span>
      </div>

      {/* Cards */}
      <div className="flex-1 overflow-y-auto p-2 space-y-2">
        <SortableContext items={sortableItems} strategy={verticalListSortingStrategy}>
          {(() => {
            // Build the list of elements to render
            const elements: React.ReactNode[] = [];
            const isReceivingCard = isOverColumn && activeCard && activeCard.column !== column.name;
            const insertIndex = overIndex !== null ? Math.min(overIndex, cards.length) : cards.length;

            // Render a ghost placeholder that matches the active card's size
            const renderPlaceholder = () => (
              <div
                key="placeholder"
                className="bg-blue-100 dark:bg-blue-900/30 border-2 border-dashed border-blue-300 dark:border-blue-700 rounded-lg p-3 opacity-70"
              >
                {/* Render same content as the card for proper sizing */}
                {activeCard && (
                  <>
                    <h3 className="font-medium text-blue-400 dark:text-blue-300 text-sm">{activeCard.title}</h3>
                    <div className="flex items-center justify-between mt-2 text-xs text-blue-300 dark:text-blue-400">
                      <span className="font-mono">{activeCard.alias}</span>
                    </div>
                  </>
                )}
              </div>
            );

            cards.forEach((card, idx) => {
              // Insert placeholder before this card if needed
              if (isReceivingCard && idx === insertIndex) {
                elements.push(renderPlaceholder());
              }

              // Show as placeholder if it's being dragged from this column
              const isBeingDragged = activeCard !== null && activeCard.id === card.id;

              elements.push(
                <CardComponent
                  key={card.id}
                  card={card}
                  board={board}
                  onClick={() => onCardClick(card)}
                  onDelete={() => onDeleteCard(card.id)}
                  isPlaceholder={isBeingDragged}
                  isHighlighted={card.id === highlightedCardId}
                />
              );
            });

            // Insert placeholder at end if needed
            if (isReceivingCard && insertIndex >= cards.length) {
              elements.push(renderPlaceholder());
            }

            return elements;
          })()}
        </SortableContext>

        {/* Add Card Form */}
        {isAddingCard && (
          <form ref={formRef} onSubmit={handleSubmit} className="bg-white dark:bg-gray-700 rounded-lg p-2 shadow-sm">
            <input
              ref={inputRef}
              type="text"
              value={draftTitle}
              onChange={(e) => onDraftChange(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Enter card title..."
              className="w-full px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-800 dark:text-white rounded focus:outline-none focus:ring-2 focus:ring-blue-500 placeholder:text-gray-400 dark:placeholder:text-gray-500"
              autoFocus
            />
            <div className="flex items-center justify-between mt-2">
              <div className="flex gap-2">
                <button
                  type="submit"
                  className="px-3 py-1 text-sm bg-blue-500 text-white rounded hover:bg-blue-600"
                >
                  Add
                </button>
                <button
                  type="button"
                  onClick={() => {
                    onDraftChange('');
                    onCancelAddCard();
                  }}
                  className="px-3 py-1 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600 rounded hover:shadow-sm transition-all"
                >
                  Cancel
                </button>
              </div>
              <span className="text-xs text-gray-400 dark:text-gray-500">⌘↵ for details</span>
            </div>
          </form>
        )}

        {/* Bottom Add Card Button - shown when not already adding */}
        {!isAddingCard && (
          <button
            onClick={onStartAddCard}
            className="w-full py-2 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-300 dark:hover:bg-gray-600 rounded transition-colors flex items-center justify-center gap-1"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            New Card
          </button>
        )}
      </div>
    </div>
  );
}
