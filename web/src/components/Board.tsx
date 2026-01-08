import { useState, useCallback, useMemo, useRef, useEffect } from 'react';
import { DndContext, DragOverlay, PointerSensor, useSensor, useSensors } from '@dnd-kit/core';
import type { DragEndEvent, DragStartEvent, DragOverEvent, CollisionDetection, DroppableContainer } from '@dnd-kit/core';
import { SortableContext, horizontalListSortingStrategy, arrayMove } from '@dnd-kit/sortable';
import type { BoardConfig, Card, Column as ColumnType, CreateCardInput, UpdateCardInput, CreateColumnInput, UpdateColumnInput } from '../api/types';
import { cardMatchesQuery } from '../utils/fuzzyMatch';
import Column from './Column';
import CardComponent from './Card';
import CardEditModal from './CardEditModal';

interface BoardProps {
  board: BoardConfig;
  cards: Card[];
  filterQuery?: string;
  highlightedCardId?: string | null;
  onMoveCard: (cardId: string, column: string, position?: number) => Promise<void>;
  onCreateCard: (input: CreateCardInput) => Promise<Card | undefined>;
  onUpdateCard: (id: string, updates: UpdateCardInput) => Promise<void>;
  onDeleteCard: (id: string) => Promise<void>;
  onCreateColumn?: (input: CreateColumnInput) => Promise<unknown>;
  onDeleteColumn?: (columnName: string) => Promise<unknown>;
  onUpdateColumn?: (columnName: string, updates: UpdateColumnInput) => Promise<unknown>;
  onReorderColumns?: (columns: string[]) => Promise<void>;
}

export default function Board({
  board,
  cards,
  filterQuery = '',
  highlightedCardId,
  onMoveCard,
  onCreateCard,
  onUpdateCard,
  onDeleteCard,
  onCreateColumn,
  onDeleteColumn,
  onUpdateColumn,
  onReorderColumns,
}: BoardProps) {
  const [activeCard, setActiveCard] = useState<Card | null>(null);
  const [activeColumn, setActiveColumn] = useState<ColumnType | null>(null);
  const [addingToColumn, setAddingToColumn] = useState<string | null>(null);
  const [isAddingColumn, setIsAddingColumn] = useState(false);
  const [newColumnName, setNewColumnName] = useState('');
  const addColumnInputRef = useRef<HTMLInputElement>(null);
  const addColumnFormRef = useRef<HTMLFormElement>(null);
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

  // Filter cards based on search query
  const filteredCards = useMemo(() => {
    if (!filterQuery.trim()) return cards;
    return cards.filter((card) => cardMatchesQuery(card, filterQuery.trim(), board));
  }, [cards, filterQuery, board]);

  const cardsByColumn = board.columns.reduce<Record<string, Card[]>>((acc, column) => {
    acc[column.name] = filteredCards.filter((card) => card.column === column.name);
    return acc;
  }, {});

  // Always show all columns (even when filtering, so cards can be moved into empty columns)
  const visibleColumns = useMemo(() => board.columns, [board.columns]);

  // Custom collision detection: behavior differs for card drags vs column drags
  const customCollisionDetection: CollisionDetection = useCallback((args) => {
    const { droppableContainers, pointerCoordinates, active } = args;

    if (!pointerCoordinates) {
      return [];
    }

    const { x: pointerX, y: pointerY } = pointerCoordinates;
    const activeId = active.id as string;
    const isDraggingColumn = columnNames.includes(activeId);

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

    // For column drags: pure X-based detection, find closest column by X center
    if (isDraggingColumn) {
      let closestColumn: DroppableContainer | null = null;
      let minDistance = Infinity;

      for (const column of columnContainers) {
        const rect = column.rect.current;
        if (!rect) continue;

        const columnCenterX = rect.left + rect.width / 2;
        const distance = Math.abs(pointerX - columnCenterX);

        if (distance < minDistance) {
          minDistance = distance;
          closestColumn = column;
        }
      }

      return closestColumn ? [{ id: closestColumn.id }] : [];
    }

    // For card drags: X determines column, Y determines position within column
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

    // Find cards in this column and determine position by Y
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
    const activeId = event.active.id as string;

    // Check if we're dragging a column
    const column = board.columns.find((c) => c.name === activeId);
    if (column) {
      setActiveColumn(column);
      return;
    }

    // Otherwise it's a card
    const card = cards.find((c) => c.id === activeId);
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
    const wasColumnDrag = activeColumn !== null;

    setActiveCard(null);
    setActiveColumn(null);
    setOverColumn(null);
    setOverIndex(null);

    const { active, over } = event;
    if (!over) return;

    const activeId = active.id as string;
    const overId = over.id as string;

    // Handle column reordering
    if (wasColumnDrag) {
      if (activeId === overId) return; // No change

      const oldIndex = columnNames.indexOf(activeId);
      const newIndex = columnNames.indexOf(overId);

      if (oldIndex !== -1 && newIndex !== -1 && oldIndex !== newIndex) {
        const newOrder = arrayMove(columnNames, oldIndex, newIndex);
        try {
          await onReorderColumns?.(newOrder);
        } catch (e) {
          console.error('Failed to reorder columns:', e);
        }
      }
      return;
    }

    // Handle card movement
    const draggedCard = cards.find((c) => c.id === activeId);
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
        const oldIndex = columnCards.findIndex((c) => c.id === activeId);
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
      await onMoveCard(activeId, targetColumn, position);
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

  const handleSaveCard = async (updates: UpdateCardInput) => {
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

  // Focus add column input when opening
  useEffect(() => {
    if (isAddingColumn && addColumnInputRef.current) {
      addColumnInputRef.current.focus();
    }
  }, [isAddingColumn]);

  // Click outside to close add column form
  useEffect(() => {
    if (!isAddingColumn) return;

    const handleClickOutside = (e: MouseEvent) => {
      if (addColumnFormRef.current && !addColumnFormRef.current.contains(e.target as Node)) {
        setIsAddingColumn(false);
        setNewColumnName('');
      }
    };

    const timeoutId = setTimeout(() => {
      document.addEventListener('mousedown', handleClickOutside);
    }, 0);

    return () => {
      clearTimeout(timeoutId);
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [isAddingColumn]);

  const handleAddColumn = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newColumnName.trim() || !onCreateColumn) return;

    try {
      await onCreateColumn({ name: newColumnName.trim().toLowerCase().replace(/\s+/g, '-') });
      setNewColumnName('');
      setIsAddingColumn(false);
    } catch (err) {
      console.error('Failed to create column:', err);
    }
  };

  return (
    <>
      <DndContext
        sensors={sensors}
        collisionDetection={customCollisionDetection}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
        <div className="flex gap-4 p-4 h-full overflow-x-auto">
          {filteredCards.length === 0 && filterQuery.trim() && (
            <div className="flex-1 flex items-center justify-center">
              <p className="text-gray-500 dark:text-gray-400">No cards match your filter</p>
            </div>
          )}
          <SortableContext items={columnNames} strategy={horizontalListSortingStrategy}>
            {visibleColumns.map((column) => (
              <Column
                key={column.name}
                column={column}
                cards={cardsByColumn[column.name] || []}
                board={board}
                highlightedCardId={highlightedCardId}
                isAddingCard={addingToColumn === column.name}
                draftTitle={draftTitles[column.name] || ''}
                onDraftChange={(title) => setDraftTitles((prev) => ({ ...prev, [column.name]: title }))}
                onStartAddCard={() => setAddingToColumn(column.name)}
                onCancelAddCard={() => setAddingToColumn(null)}
                onAddCard={(title, openModal, keepFormOpen) => handleAddCard(column.name, title, openModal, keepFormOpen)}
                onCardClick={handleCardClick}
                onDeleteCard={handleDeleteCard}
                onDeleteColumn={onDeleteColumn}
                onUpdateColumn={onUpdateColumn}
                activeCard={activeCard}
                isOverColumn={overColumn === column.name}
                overIndex={overColumn === column.name ? overIndex : null}
                isDragging={activeColumn?.name === column.name}
              />
            ))}
          </SortableContext>

          {/* Add Column Button/Form */}
          {onCreateColumn && !filterQuery.trim() && (
            <div className="flex-1 min-w-64 max-w-sm">
              {isAddingColumn ? (
                <form
                  ref={addColumnFormRef}
                  onSubmit={handleAddColumn}
                  className="bg-gray-200 dark:bg-gray-800 rounded-lg p-3"
                >
                  <input
                    ref={addColumnInputRef}
                    type="text"
                    value={newColumnName}
                    onChange={(e) => setNewColumnName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Escape') {
                        setIsAddingColumn(false);
                        setNewColumnName('');
                      }
                    }}
                    placeholder="Column name..."
                    className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded focus:outline-none focus:ring-2 focus:ring-blue-500 placeholder:text-gray-400 dark:placeholder:text-gray-500"
                  />
                  <div className="flex gap-2 mt-2">
                    <button
                      type="submit"
                      disabled={!newColumnName.trim()}
                      className="px-3 py-1 text-sm bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      Add
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setIsAddingColumn(false);
                        setNewColumnName('');
                      }}
                      className="px-3 py-1 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
                    >
                      Cancel
                    </button>
                  </div>
                </form>
              ) : (
                <button
                  onClick={() => setIsAddingColumn(true)}
                  className="w-full py-3 px-4 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 bg-gray-200/50 dark:bg-gray-800/50 hover:bg-gray-300 dark:hover:bg-gray-700 rounded-lg transition-colors flex items-center justify-center gap-2"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                  </svg>
                  Add Column
                </button>
              )}
            </div>
          )}
        </div>
        <DragOverlay>
          {activeCard ? (
            <CardComponent card={activeCard} board={board} isDragging />
          ) : activeColumn ? (
            <div className="bg-gray-200 dark:bg-gray-800 rounded-lg shadow-lg opacity-90 cursor-grabbing">
              <div className="flex items-center gap-2 px-3 py-2 border-b border-gray-300 dark:border-gray-600">
                <div
                  className="w-3 h-3 rounded-full flex-shrink-0"
                  style={{ backgroundColor: activeColumn.color }}
                />
                <h2 className="font-medium text-gray-700 dark:text-gray-200 truncate">
                  {activeColumn.name}
                </h2>
              </div>
            </div>
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
