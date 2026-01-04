import { useState } from 'react';
import { DndContext, DragOverlay, closestCenter, PointerSensor, useSensor, useSensors } from '@dnd-kit/core';
import type { DragEndEvent, DragStartEvent } from '@dnd-kit/core';
import type { BoardConfig, Card, CreateCardInput } from '../api/types';
import Column from './Column';
import CardComponent from './Card';

interface BoardProps {
  board: BoardConfig;
  cards: Card[];
  onMoveCard: (cardId: string, column: string) => Promise<void>;
  onCreateCard: (input: CreateCardInput) => Promise<Card | undefined>;
}

export default function Board({ board, cards, onMoveCard, onCreateCard }: BoardProps) {
  const [activeCard, setActiveCard] = useState<Card | null>(null);
  const [addingToColumn, setAddingToColumn] = useState<string | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    })
  );

  const cardsByColumn = board.columns.reduce<Record<string, Card[]>>((acc, column) => {
    acc[column.name] = cards.filter((card) => card.column === column.name);
    return acc;
  }, {});

  const handleDragStart = (event: DragStartEvent) => {
    const card = cards.find((c) => c.id === event.active.id);
    if (card) {
      setActiveCard(card);
    }
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    setActiveCard(null);

    const { active, over } = event;
    if (!over) return;

    const cardId = active.id as string;
    const targetColumn = over.id as string;

    const card = cards.find((c) => c.id === cardId);
    if (!card || card.column === targetColumn) return;

    // Check if dropping on a valid column
    if (!board.columns.some((col) => col.name === targetColumn)) return;

    try {
      await onMoveCard(cardId, targetColumn);
    } catch (e) {
      console.error('Failed to move card:', e);
    }
  };

  const handleAddCard = async (column: string, title: string) => {
    try {
      await onCreateCard({ title, column });
      setAddingToColumn(null);
    } catch (e) {
      console.error('Failed to create card:', e);
    }
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
    >
      <div className="flex gap-4 p-4 h-full overflow-x-auto">
        {board.columns.map((column) => (
          <Column
            key={column.name}
            column={column}
            cards={cardsByColumn[column.name] || []}
            labels={board.labels || []}
            isAddingCard={addingToColumn === column.name}
            onStartAddCard={() => setAddingToColumn(column.name)}
            onCancelAddCard={() => setAddingToColumn(null)}
            onAddCard={(title) => handleAddCard(column.name, title)}
          />
        ))}
      </div>
      <DragOverlay>
        {activeCard ? (
          <CardComponent card={activeCard} labels={board.labels || []} isDragging />
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
