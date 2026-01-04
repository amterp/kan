import { useState, useMemo } from 'react';
import { useDroppable } from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import type { Card, Column as ColumnType, Label } from '../api/types';
import CardComponent from './Card';

interface ColumnProps {
  column: ColumnType;
  cards: Card[];
  labels: Label[];
  isAddingCard: boolean;
  onStartAddCard: () => void;
  onCancelAddCard: () => void;
  onAddCard: (title: string, openModal: boolean) => void;
  onCardClick: (card: Card) => void;
  activeCard: Card | null;
  isOverColumn: boolean;
  overIndex: number | null;
}

export default function Column({
  column,
  cards,
  labels,
  isAddingCard,
  onStartAddCard,
  onCancelAddCard,
  onAddCard,
  onCardClick,
  activeCard,
  isOverColumn,
  overIndex,
}: ColumnProps) {
  const [newCardTitle, setNewCardTitle] = useState('');
  const { setNodeRef, isOver } = useDroppable({ id: column.name });

  // SortableContext items - just the cards in this column
  // Don't add activeCard for cross-column drags (we use a manual placeholder instead)
  const sortableItems = useMemo(() => {
    return cards.map((c) => c.id);
  }, [cards]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (newCardTitle.trim()) {
      onAddCard(newCardTitle.trim(), false);
      setNewCardTitle('');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setNewCardTitle('');
      onCancelAddCard();
    } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      // Cmd+Enter or Ctrl+Enter - create and open modal
      e.preventDefault();
      if (newCardTitle.trim()) {
        onAddCard(newCardTitle.trim(), true);
        setNewCardTitle('');
      }
    }
  };

  return (
    <div
      ref={setNodeRef}
      className={`flex-1 min-w-64 max-w-sm flex flex-col bg-gray-200 rounded-lg max-h-full ${
        isOver ? 'ring-2 ring-blue-400' : ''
      }`}
    >
      {/* Column Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-300">
        <div className="flex items-center gap-2">
          <div
            className="w-3 h-3 rounded-full"
            style={{ backgroundColor: column.color }}
          />
          <h2 className="font-medium text-gray-700">{column.name}</h2>
          <span className="text-sm text-gray-500">{cards.length}</span>
        </div>
        <button
          onClick={onStartAddCard}
          className="text-gray-400 hover:text-gray-600 p-1 rounded hover:bg-gray-300"
          title="Add card"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
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
                className="bg-blue-100 border-2 border-dashed border-blue-300 rounded-lg p-3 opacity-70"
              >
                {/* Render same content as the card for proper sizing */}
                {activeCard && (
                  <>
                    {activeCard.labels && activeCard.labels.length > 0 && (
                      <div className="flex flex-wrap gap-1 mb-2">
                        {activeCard.labels.map((labelName) => {
                          const label = labels.find((l) => l.name === labelName);
                          return label ? (
                            <span
                              key={label.name}
                              className="px-2 py-0.5 text-xs rounded-full text-white opacity-50"
                              style={{ backgroundColor: label.color }}
                            >
                              {label.name}
                            </span>
                          ) : null;
                        })}
                      </div>
                    )}
                    <h3 className="font-medium text-blue-400 text-sm">{activeCard.title}</h3>
                    <div className="flex items-center justify-between mt-2 text-xs text-blue-300">
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
                  labels={labels}
                  onClick={() => onCardClick(card)}
                  isPlaceholder={isBeingDragged}
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

        {/* Empty state */}
        {cards.length === 0 && !isAddingCard && !isOverColumn && (
          <div className="text-center py-8 text-gray-400 text-sm">
            No cards yet
          </div>
        )}

        {/* Add Card Form */}
        {isAddingCard && (
          <form onSubmit={handleSubmit} className="bg-white rounded-lg p-2 shadow-sm">
            <input
              type="text"
              value={newCardTitle}
              onChange={(e) => setNewCardTitle(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Enter card title..."
              className="w-full px-2 py-1 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
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
                    setNewCardTitle('');
                    onCancelAddCard();
                  }}
                  className="px-3 py-1 text-sm text-gray-600 hover:text-gray-800"
                >
                  Cancel
                </button>
              </div>
              <span className="text-xs text-gray-400">⌘↵ for details</span>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
