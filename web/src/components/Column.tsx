import { useState } from 'react';
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
  onAddCard: (title: string) => void;
}

export default function Column({
  column,
  cards,
  labels,
  isAddingCard,
  onStartAddCard,
  onCancelAddCard,
  onAddCard,
}: ColumnProps) {
  const [newCardTitle, setNewCardTitle] = useState('');
  const { setNodeRef, isOver } = useDroppable({ id: column.name });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (newCardTitle.trim()) {
      onAddCard(newCardTitle.trim());
      setNewCardTitle('');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setNewCardTitle('');
      onCancelAddCard();
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
        <SortableContext items={cards.map((c) => c.id)} strategy={verticalListSortingStrategy}>
          {cards.map((card) => (
            <CardComponent key={card.id} card={card} labels={labels} />
          ))}
        </SortableContext>

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
            <div className="flex gap-2 mt-2">
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
          </form>
        )}
      </div>
    </div>
  );
}
