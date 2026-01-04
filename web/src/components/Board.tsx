import { useState, useCallback } from 'react';
import { DndContext, DragOverlay, PointerSensor, useSensor, useSensors } from '@dnd-kit/core';
import type { DragEndEvent, DragStartEvent, DragOverEvent, CollisionDetection, DroppableContainer } from '@dnd-kit/core';
import type { BoardConfig, Card, CreateCardInput } from '../api/types';
import Column from './Column';
import CardComponent from './Card';
import CardEditModal from './CardEditModal';

interface BoardProps {
  board: BoardConfig;
  cards: Card[];
  onMoveCard: (cardId: string, column: string, position?: number) => Promise<void>;
  onCreateCard: (input: CreateCardInput) => Promise<Card | undefined>;
  onUpdateCard: (id: string, updates: Partial<Card>) => Promise<void>;
  onDeleteCard: (id: string) => Promise<void>;
}

export default function Board({ board, cards, onMoveCard, onCreateCard, onUpdateCard, onDeleteCard }: BoardProps) {
  const [activeCard, setActiveCard] = useState<Card | null>(null);
  const [addingToColumn, setAddingToColumn] = useState<string | null>(null);
  const [editingCard, setEditingCard] = useState<Card | null>(null);
  const [newCardForEdit, setNewCardForEdit] = useState<{ card: Card; column: string } | null>(null);

  // Draft titles per column (preserved when clicking outside)
  const [draftTitles, setDraftTitles] = useState<Record<string, string>>({});

  // Track which column is being hovered and at what position for cross-column drag preview
  const [overColumn, setOverColumn] = useState<string | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    })
  );

  const columnNames = board.columns.map((c) => c.name);

  const cardsByColumn = board.columns.reduce<Record<string, Card[]>>((acc, column) => {
    acc[column.name] = cards.filter((card) => card.column === column.name);
    return acc;
  }, {});

  // Custom collision detection: X determines column, Y determines position within column
  const xBasedCollisionDetection: CollisionDetection = useCallback((args) => {
    const { droppableContainers, pointerCoordinates } = args;

    if (!pointerCoordinates) {
      return [];
    }

    const { x: pointerX, y: pointerY } = pointerCoordinates;

    // Separate containers into columns and cards
    const columnContainers: DroppableContainer[] = [];
    const cardContainers: DroppableContainer[] = [];

    droppableContainers.forEach((container) => {
      if (columnNames.includes(container.id as string)) {
        columnContainers.push(container);
      } else {
        cardContainers.push(container);
      }
    });

    // Step 1: Find which column the pointer X is over
    let targetColumn: DroppableContainer | null = null;
    let minXDistance = Infinity;

    for (const column of columnContainers) {
      const rect = column.rect.current;
      if (!rect) continue;

      // Check if pointer X is within column bounds
      if (pointerX >= rect.left && pointerX <= rect.right) {
        targetColumn = column;
        break;
      }

      // Otherwise find closest column by X
      const distToLeft = Math.abs(pointerX - rect.left);
      const distToRight = Math.abs(pointerX - rect.right);
      const dist = Math.min(distToLeft, distToRight);
      if (dist < minXDistance) {
        minXDistance = dist;
        targetColumn = column;
      }
    }

    if (!targetColumn) {
      return [];
    }

    const targetColumnName = targetColumn.id as string;

    // Step 2: Find cards in this column and determine position by Y
    const cardsInColumn = cardContainers.filter((container) => {
      const cardId = container.id as string;
      const card = cards.find((c) => c.id === cardId);
      return card && card.column === targetColumnName;
    });

    if (cardsInColumn.length === 0) {
      // Empty column - return the column itself
      return [{ id: targetColumn.id }];
    }

    // Sort cards by Y position (top to bottom)
    const sortedCards = [...cardsInColumn].sort((a, b) => {
      const rectA = a.rect.current;
      const rectB = b.rect.current;
      if (!rectA || !rectB) return 0;
      return rectA.top - rectB.top;
    });

    // Find insertion point: the first card whose center is BELOW the pointer
    // This means: if pointer is above card N's center, insert before card N
    for (const cardContainer of sortedCards) {
      const rect = cardContainer.rect.current;
      if (!rect) continue;

      const cardCenterY = rect.top + rect.height / 2;
      if (pointerY < cardCenterY) {
        // Pointer is above this card's center - insert before this card
        return [{ id: cardContainer.id }];
      }
    }

    // Pointer is below all cards' centers - append to end (return column)
    return [{ id: targetColumn.id }];
  }, [columnNames, cards]);

  const handleDragStart = (event: DragStartEvent) => {
    const card = cards.find((c) => c.id === event.active.id);
    if (card) {
      setActiveCard(card);
    }
  };

  const handleDragOver = (event: DragOverEvent) => {
    const { over } = event;
    if (!over || !activeCard) {
      setOverColumn(null);
      setOverIndex(null);
      return;
    }

    const overId = over.id as string;
    const isColumn = columnNames.includes(overId);

    if (isColumn) {
      // Hovering over column itself (empty area)
      setOverColumn(overId);
      setOverIndex(cardsByColumn[overId]?.length ?? 0);
    } else {
      // Hovering over a card
      const targetCard = cards.find((c) => c.id === overId);
      if (targetCard) {
        setOverColumn(targetCard.column);
        const columnCards = cardsByColumn[targetCard.column] || [];
        const idx = columnCards.findIndex((c) => c.id === overId);
        setOverIndex(idx);
      }
    }
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    setActiveCard(null);
    setOverColumn(null);
    setOverIndex(null);

    const { active, over } = event;
    if (!over) return;

    const cardId = active.id as string;
    const overId = over.id as string;

    const draggedCard = cards.find((c) => c.id === cardId);
    if (!draggedCard) return;

    // Determine if we dropped on a column or a card
    const isColumn = columnNames.includes(overId);

    let targetColumn: string;
    let position: number | undefined;

    if (isColumn) {
      // Dropped on a column (empty area or column header)
      targetColumn = overId;
      // Append to end
      position = undefined;
    } else {
      // Dropped on a card
      const targetCard = cards.find((c) => c.id === overId);
      if (!targetCard) return;

      targetColumn = targetCard.column;
      const columnCards = cardsByColumn[targetColumn] || [];
      const targetIndex = columnCards.findIndex((c) => c.id === overId);

      if (draggedCard.column === targetColumn) {
        // Same column reorder
        const oldIndex = columnCards.findIndex((c) => c.id === cardId);
        if (oldIndex === targetIndex) return; // No change

        // Calculate new position
        // If moving down (oldIndex < targetIndex), we want to be at targetIndex
        // If moving up (oldIndex > targetIndex), we want to be at targetIndex
        position = targetIndex;
      } else {
        // Different column - insert at target card's position
        position = targetIndex;
      }
    }

    // Skip if same column and no position (no real move)
    if (draggedCard.column === targetColumn && position === undefined) return;

    try {
      await onMoveCard(cardId, targetColumn, position);
    } catch (e) {
      console.error('Failed to move card:', e);
    }
  };

  const handleAddCard = async (column: string, title: string, openModal: boolean, keepFormOpen?: boolean) => {
    try {
      const newCard = await onCreateCard({ title, column });
      // Only close the form if not keeping it open for continuous add
      if (!keepFormOpen) {
        setAddingToColumn(null);
      }
      if (openModal && newCard) {
        setNewCardForEdit({ card: newCard, column });
      }
    } catch (e) {
      console.error('Failed to create card:', e);
    }
  };

  const handleCardClick = (card: Card) => {
    setEditingCard(card);
  };

  const handleSaveCard = async (updates: Partial<Card>) => {
    if (editingCard) {
      await onUpdateCard(editingCard.id, updates);
      setEditingCard(null);
    } else if (newCardForEdit) {
      await onUpdateCard(newCardForEdit.card.id, updates);
      setNewCardForEdit(null);
    }
  };

  const handleCloseModal = () => {
    setEditingCard(null);
    setNewCardForEdit(null);
  };

  const handleDeleteCard = async (cardId: string) => {
    await onDeleteCard(cardId);
    handleCloseModal();
  };

  const currentEditCard = editingCard || newCardForEdit?.card || null;

  return (
    <>
      <DndContext
        sensors={sensors}
        collisionDetection={xBasedCollisionDetection}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
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
              draftTitle={draftTitles[column.name] || ''}
              onDraftChange={(title) => setDraftTitles((prev) => ({ ...prev, [column.name]: title }))}
              onStartAddCard={() => setAddingToColumn(column.name)}
              onCancelAddCard={() => setAddingToColumn(null)}
              onAddCard={(title, openModal, keepFormOpen) => handleAddCard(column.name, title, openModal, keepFormOpen)}
              onCardClick={handleCardClick}
              onDeleteCard={handleDeleteCard}
              activeCard={activeCard}
              isOverColumn={overColumn === column.name}
              overIndex={overColumn === column.name ? overIndex : null}
            />
          ))}
        </div>
        <DragOverlay>
          {activeCard ? (
            <CardComponent card={activeCard} labels={board.labels || []} isDragging />
          ) : null}
        </DragOverlay>
      </DndContext>

      {currentEditCard && (
        <CardEditModal
          card={currentEditCard}
          board={board}
          onSave={handleSaveCard}
          onDelete={() => handleDeleteCard(currentEditCard.id)}
          onClose={handleCloseModal}
        />
      )}
    </>
  );
}
