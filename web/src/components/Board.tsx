import { useState, useCallback, useMemo, useRef, useEffect } from 'react';
import { DndContext, DragOverlay, PointerSensor, useSensor, useSensors } from '@dnd-kit/core';
import type { DragEndEvent, DragStartEvent, DragOverEvent, CollisionDetection, DroppableContainer } from '@dnd-kit/core';
import { SortableContext, horizontalListSortingStrategy, verticalListSortingStrategy, arrayMove } from '@dnd-kit/sortable';
import type { BoardConfig, Card, Column as ColumnType, CreateCardInput, CreateCardResponse, UpdateCardInput, CreateColumnInput, UpdateColumnInput } from '../api/types';
import type { UndoAction } from '../hooks/useUndo';
import { cardMatchesQuery } from '../utils/fuzzyMatch';
import { toApiFieldValue } from '../utils/customFields';
import { BoardConfigProvider } from '../contexts/BoardConfigContext';
import { useToast } from '../contexts/ToastContext';
import Column from './Column';
import CardComponent from './Card';
import FloatingFieldPanel from './FloatingFieldPanel';
import CardContextMenu from './CardContextMenu';
import { useSlimMode } from '../contexts/SlimModeContext';

// Panel target - tracks what the floating field panel is editing
interface PanelTarget {
  type: 'draft' | 'created';
  column: string;
  cardId?: string; // Only set when type is 'created'
  anchorEl: HTMLElement;
}

interface BoardProps {
  board: BoardConfig;
  cards: Card[];
  filterQuery?: string;
  highlightedCardId?: string | null;
  onMoveCard: (cardId: string, column: string, position?: number) => Promise<void>;
  onCreateCard: (input: CreateCardInput) => Promise<CreateCardResponse | undefined>;
  onUpdateCard: (id: string, updates: UpdateCardInput) => Promise<void>;
  onDeleteCard: (id: string) => Promise<void>;
  onCreateColumn?: (input: CreateColumnInput) => Promise<unknown>;
  onDeleteColumn?: (columnName: string) => Promise<unknown>;
  onUpdateColumn?: (columnName: string, updates: UpdateColumnInput) => Promise<unknown>;
  onReorderColumns?: (columns: string[]) => Promise<void>;
  onOpenCard: (cardId: string, focusDescription?: boolean) => void;
  onPushUndo?: (action: UndoAction) => void;
  isOmnibarOpen?: boolean;
  isCardModalOpen?: boolean;
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
  onOpenCard,
  onPushUndo,
  isOmnibarOpen = false,
  isCardModalOpen = false,
}: BoardProps) {
  const { showToast } = useToast();
  const { isSlim } = useSlimMode();
  const [activeCard, setActiveCard] = useState<Card | null>(null);
  const [activeColumn, setActiveColumn] = useState<ColumnType | null>(null);
  const [addingToColumn, setAddingToColumn] = useState<string | null>(null);
  const [isAddingColumn, setIsAddingColumn] = useState(false);
  const [newColumnName, setNewColumnName] = useState('');
  const addColumnInputRef = useRef<HTMLInputElement>(null);
  const addColumnFormRef = useRef<HTMLFormElement>(null);
  // Track which column name is being edited (lifted from Column for stability across re-renders)
  const [editingColumnName, setEditingColumnName] = useState<string | null>(null);

  // Draft titles per column (preserved when clicking outside)
  const [draftTitles, setDraftTitles] = useState<Record<string, string>>({});

  // Panel target - what the floating field panel is currently editing
  const [panelTarget, setPanelTarget] = useState<PanelTarget | null>(null);

  // Debounce timer for updating created cards
  const updateTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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

  const cardsByColumn = useMemo(() =>
    board.columns.reduce<Record<string, Card[]>>((acc, column) => {
      acc[column.name] = filteredCards.filter((card) => card.column === column.name);
      return acc;
    }, {}),
    [board.columns, filteredCards]
  );

  // Unfiltered card counts for column limit checks (filter must not bypass limits)
  const allCardsByColumn = useMemo(() =>
    board.columns.reduce<Record<string, Card[]>>((acc, column) => {
      acc[column.name] = cards.filter((card) => card.column === column.name);
      return acc;
    }, {}),
    [board.columns, cards]
  );

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

    // For column drags: find closest column along the layout axis
    if (isDraggingColumn) {
      let closestColumn: DroppableContainer | null = null;
      let minDistance = Infinity;

      for (const column of columnContainers) {
        const rect = column.rect.current;
        if (!rect) continue;

        // Slim mode: Y-based detection; normal mode: X-based detection
        const center = isSlim
          ? rect.top + rect.height / 2
          : rect.left + rect.width / 2;
        const pointer = isSlim ? pointerY : pointerX;
        const distance = Math.abs(pointer - center);

        if (distance < minDistance) {
          minDistance = distance;
          closestColumn = column;
        }
      }

      return closestColumn ? [{ id: closestColumn.id }] : [];
    }

    // For card drags: primary axis determines column, Y determines position within column
    let targetColumn: DroppableContainer | null = null;
    let minAxisDistance = Infinity;

    for (const column of columnContainers) {
      const rect = column.rect.current;
      if (!rect) continue;

      if (isSlim) {
        // Slim mode: Y determines target column
        if (pointerY >= rect.top && pointerY <= rect.bottom) {
          targetColumn = column;
          break;
        }
        const distToTop = Math.abs(pointerY - rect.top);
        const distToBottom = Math.abs(pointerY - rect.bottom);
        const dist = Math.min(distToTop, distToBottom);
        if (dist < minAxisDistance) {
          minAxisDistance = dist;
          targetColumn = column;
        }
      } else {
        // Normal mode: X determines target column
        if (pointerX >= rect.left && pointerX <= rect.right) {
          targetColumn = column;
          break;
        }
        const distToLeft = Math.abs(pointerX - rect.left);
        const distToRight = Math.abs(pointerX - rect.right);
        const dist = Math.min(distToLeft, distToRight);
        if (dist < minAxisDistance) {
          minAxisDistance = dist;
          targetColumn = column;
        }
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
  }, [columnNames, cards, isSlim]);

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

    // Client-side column limit check to avoid the jarring optimistic-then-revert UX
    if (draggedCard.column !== targetColumn) {
      const col = board.columns.find((c) => c.name === targetColumn);
      if (col?.limit) {
        const colCards = allCardsByColumn[targetColumn] || [];
        if (colCards.length >= col.limit) {
          showToast('error', `Column "${targetColumn}" is full (limit: ${col.limit})`);
          return;
        }
      }
    }

    // Capture position data before the move (optimistic update changes it)
    const fromColumnCards = allCardsByColumn[draggedCard.column] || [];
    const fromPosition = Math.max(0, fromColumnCards.findIndex((c) => c.id === activeId));

    try {
      await onMoveCard(activeId, targetColumn, position);
      // Push undo only after successful move
      onPushUndo?.({
        type: 'move',
        cardId: activeId,
        fromColumn: draggedCard.column,
        fromPosition,
        toColumn: targetColumn,
      });
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to move card';
      showToast('error', message);
    }
  };

  // Local state for created card fields (to avoid stale data issues)
  const [createdCardFields, setCreatedCardFields] = useState<Record<string, unknown>>({});

  const handleAddCard = async (column: string, title: string, openModal: boolean, keepFormOpen?: boolean, showPanel?: boolean) => {
    try {
      const response = await onCreateCard({ title, column });
      const newCard = response?.card;

      // Close the form if not keeping it open
      if (!keepFormOpen) {
        setAddingToColumn(null);
      }

      if (openModal && newCard) {
        setPanelTarget(null);
        onOpenCard(newCard.id, true);
      } else if (showPanel && newCard) {
        // Show panel anchored to the just-created card.
        // NOTE: This relies on Card.tsx rendering with data-card-id={card.id} attribute.
        // We use double RAF to wait for React to render the new card after state update.
        setCreatedCardFields({});
        requestAnimationFrame(() => {
          requestAnimationFrame(() => {
            const cardEl = document.querySelector(`[data-card-id="${newCard.id}"]`) as HTMLElement | null;
            if (cardEl) {
              setPanelTarget({ type: 'created', column, cardId: newCard.id, anchorEl: cardEl });
            } else {
              // Card element not found - this can happen if rendering is delayed or
              // the Card component doesn't have the data-card-id attribute
              console.warn(`FloatingFieldPanel: Could not find card element for id "${newCard.id}"`);
            }
          });
        });
      } else {
        // Plain Enter - dismiss any existing panel
        setPanelTarget(null);
      }
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to create card';
      showToast('error', message);
    }
  };

  const handlePanelHide = useCallback(() => {
    // Clear any pending debounced update
    if (updateTimerRef.current) {
      clearTimeout(updateTimerRef.current);
      updateTimerRef.current = null;
    }
    setPanelTarget(null);
    setCreatedCardFields({});
  }, []);

  const handlePanelFieldChange = useCallback(async (fieldName: string, value: unknown) => {
    if (!panelTarget || panelTarget.type !== 'created' || !panelTarget.cardId) return;

    // Update local state immediately for responsive UI
    setCreatedCardFields((prev) => ({ ...prev, [fieldName]: value }));

    const apiValue = toApiFieldValue(value);
    const cardId = panelTarget.cardId;

    // Check field type - only debounce string fields (user typing)
    const fieldType = board.custom_fields?.[fieldName]?.type;
    const shouldDebounce = fieldType === 'string';

    if (shouldDebounce) {
      // Debounce text input
      if (updateTimerRef.current) {
        clearTimeout(updateTimerRef.current);
      }
      updateTimerRef.current = setTimeout(async () => {
        try {
          await onUpdateCard(cardId, { custom_fields: { [fieldName]: apiValue } });
        } catch (e) {
          console.error('Failed to update card field:', e);
        }
      }, 500);
    } else {
      // Immediate update for enum, tags, date
      try {
        await onUpdateCard(cardId, { custom_fields: { [fieldName]: apiValue } });
      } catch (e) {
        console.error('Failed to update card field:', e);
      }
    }
  }, [panelTarget, onUpdateCard, board.custom_fields]);

  // Compute field values for the panel
  const panelFieldValues = useMemo((): Record<string, unknown> => {
    if (!panelTarget || panelTarget.type !== 'created') return {};

    // Merge card data with local edits (local edits take precedence)
    const card = cards.find((c) => c.id === panelTarget.cardId);
    const cardValues: Record<string, unknown> = {};
    if (card && board.custom_fields) {
      for (const fieldName of Object.keys(board.custom_fields)) {
        if (card[fieldName] !== undefined) {
          cardValues[fieldName] = card[fieldName];
        }
      }
    }
    return { ...cardValues, ...createdCardFields };
  }, [panelTarget, cards, board.custom_fields, createdCardFields]);

  const handleCardClick = (card: Card) => {
    if (isSlim) return; // Slim mode: no card modal, quick-processing only
    setPanelTarget(null); // Dismiss panel when opening card modal
    onOpenCard(card.id);
  };

  // Advance card to the next column (slim mode)
  const handleAdvanceCard = useCallback(async (columnName: string, cardId: string) => {
    const colIndex = board.columns.findIndex((c) => c.name === columnName);
    if (colIndex === -1 || colIndex >= board.columns.length - 1) return;

    const nextColumn = board.columns[colIndex + 1];

    // Check column limit (use unfiltered count - filter must not bypass limits)
    if (nextColumn.limit) {
      const nextColumnCards = allCardsByColumn[nextColumn.name] || [];
      if (nextColumnCards.length >= nextColumn.limit) {
        showToast('error', `Column "${nextColumn.name}" is full (limit: ${nextColumn.limit})`);
        return;
      }
    }

    // Capture position before the move
    const fromColumnCards = allCardsByColumn[columnName] || [];
    const fromPosition = Math.max(0, fromColumnCards.findIndex((c) => c.id === cardId));

    try {
      await onMoveCard(cardId, nextColumn.name, 0);
      onPushUndo?.({
        type: 'move',
        cardId,
        fromColumn: columnName,
        fromPosition,
        toColumn: nextColumn.name,
      });
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to advance card';
      showToast('error', message);
    }
  }, [board.columns, allCardsByColumn, onMoveCard, onPushUndo, showToast]);

  // Context menu state for slim mode
  const [contextMenu, setContextMenu] = useState<{
    cardId: string;
    cardColumn: string;
    x: number;
    y: number;
  } | null>(null);

  // Wrap onDeleteCard to capture undo data and show toast
  const handleDeleteCard = useCallback(async (cardId: string) => {
    // Capture position info while card still exists in state
    const card = cards.find((c) => c.id === cardId);
    const undoData = card ? {
      type: 'delete' as const,
      card: { ...card },
      column: card.column,
      position: Math.max(0, (allCardsByColumn[card.column] || []).findIndex((c) => c.id === cardId)),
    } : null;

    try {
      await onDeleteCard(cardId);
      // Push undo only after successful delete
      if (undoData && onPushUndo) {
        onPushUndo(undoData);
      }
      showToast('info', 'Card deleted \u2013 Cmd+Z to undo');
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to delete card';
      showToast('error', message);
    }
  }, [cards, allCardsByColumn, onDeleteCard, onPushUndo, showToast]);

  const handleCardContextMenu = useCallback((card: Card, e: React.MouseEvent) => {
    setContextMenu({
      cardId: card.id,
      cardColumn: card.column,
      x: e.clientX,
      y: e.clientY,
    });
  }, []);

  const handleContextMenuMove = useCallback(async (columnName: string) => {
    if (!contextMenu) return;
    if (columnName === contextMenu.cardColumn) {
      setContextMenu(null);
      return;
    }

    // Check column limit (use unfiltered count - filter must not bypass limits)
    const col = board.columns.find((c) => c.name === columnName);
    if (col?.limit) {
      const colCards = allCardsByColumn[columnName] || [];
      if (colCards.length >= col.limit) {
        showToast('error', `Column "${columnName}" is full (limit: ${col.limit})`);
        setContextMenu(null);
        return;
      }
    }

    // Capture position before the move
    const fromColumnCards = allCardsByColumn[contextMenu.cardColumn] || [];
    const fromPosition = Math.max(0, fromColumnCards.findIndex((c) => c.id === contextMenu.cardId));

    try {
      await onMoveCard(contextMenu.cardId, columnName);
      onPushUndo?.({
        type: 'move',
        cardId: contextMenu.cardId,
        fromColumn: contextMenu.cardColumn,
        fromPosition,
        toColumn: columnName,
      });
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Failed to move card';
      showToast('error', message);
    }
    setContextMenu(null);
  }, [contextMenu, board.columns, allCardsByColumn, onMoveCard, onPushUndo, showToast]);

  // Clear debounce timer when panel target changes (switching cards or dismissing)
  useEffect(() => {
    if (updateTimerRef.current) {
      clearTimeout(updateTimerRef.current);
      updateTimerRef.current = null;
    }
  }, [panelTarget?.cardId]);

  // Cleanup debounce timer on unmount
  useEffect(() => {
    return () => {
      if (updateTimerRef.current) {
        clearTimeout(updateTimerRef.current);
      }
    };
  }, []);

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

  // Number key shortcuts: press 1-9 to start adding a card to column N
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return;

      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if ((e.target as HTMLElement)?.isContentEditable) return;

      if (isOmnibarOpen || isCardModalOpen) return;

      const digit = parseInt(e.key, 10);
      if (isNaN(digit) || digit < 1 || digit > 9) return;

      const columnIndex = digit - 1;
      if (columnIndex >= visibleColumns.length) return;

      e.preventDefault();
      setPanelTarget(null);
      setAddingToColumn(visibleColumns[columnIndex].name);
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [visibleColumns, isOmnibarOpen, isCardModalOpen]);

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
    <BoardConfigProvider board={board}>
      <DndContext
        sensors={sensors}
        collisionDetection={customCollisionDetection}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
        <div className={isSlim
          ? "flex flex-col gap-4 p-4 h-full overflow-y-auto"
          : "flex gap-4 p-4 h-full overflow-x-auto"
        }>
          {filteredCards.length === 0 && filterQuery.trim() && (
            <div className="flex-1 flex items-center justify-center">
              <p className="text-gray-500 dark:text-gray-400">No cards match your filter</p>
            </div>
          )}
          <SortableContext items={columnNames} strategy={isSlim ? verticalListSortingStrategy : horizontalListSortingStrategy}>
            {visibleColumns.map((column, index) => (
              <Column
                key={column.name}
                column={column}
                columnIndex={index}
                cards={cardsByColumn[column.name] || []}
                board={board}
                highlightedCardId={highlightedCardId}
                isAddingCard={addingToColumn === column.name}
                draftTitle={draftTitles[column.name] || ''}
                onDraftChange={(title) => setDraftTitles((prev) => ({ ...prev, [column.name]: title }))}
                onStartAddCard={() => setAddingToColumn(column.name)}
                onCancelAddCard={() => setAddingToColumn(null)}
                onAddCard={(title, openModal, keepFormOpen, showPanel) => handleAddCard(column.name, title, openModal, keepFormOpen, showPanel)}
                onCardClick={handleCardClick}
                onDeleteCard={handleDeleteCard}
                onDeleteColumn={onDeleteColumn}
                onUpdateColumn={onUpdateColumn}
                isEditingName={editingColumnName === column.name}
                onStartEditName={() => setEditingColumnName(column.name)}
                onStopEditName={() => setEditingColumnName(null)}
                activeCard={activeCard}
                isOverColumn={overColumn === column.name}
                overIndex={overColumn === column.name ? overIndex : null}
                isDragging={activeColumn?.name === column.name}
                onPanelHide={handlePanelHide}
                onAdvanceCard={index < board.columns.length - 1
                  ? (cardId) => handleAdvanceCard(column.name, cardId)
                  : undefined}
                onCardContextMenu={isSlim ? handleCardContextMenu : undefined}
              />
            ))}
          </SortableContext>

          {/* Add Column Button/Form */}
          {onCreateColumn && !filterQuery.trim() && (
            <div className={isSlim ? "w-full" : "flex-1 min-w-64 max-w-sm"}>
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
            <div className="bg-gray-200 dark:bg-gray-800 rounded-lg shadow-lg opacity-90 cursor-grabbing border-t-[3px]" style={{ borderTopColor: activeColumn.color }}>
              <div className="flex items-center gap-2 px-3 py-2 border-b border-gray-300 dark:border-gray-600">
                <h2 className="font-semibold text-gray-700 dark:text-gray-200 truncate">
                  {activeColumn.name}
                </h2>
              </div>
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>

      {/* Floating field panel for quick card creation */}
      {panelTarget && (
        <FloatingFieldPanel
          board={board}
          values={panelFieldValues}
          onChange={handlePanelFieldChange}
          anchorEl={panelTarget.anchorEl}
          onDismiss={handlePanelHide}
        />
      )}

      {/* Context menu for moving cards (slim mode) */}
      {contextMenu && (
        <CardContextMenu
          columns={board.columns}
          currentColumn={contextMenu.cardColumn}
          x={contextMenu.x}
          y={contextMenu.y}
          onMove={handleContextMenuMove}
          onClose={() => setContextMenu(null)}
        />
      )}

    </BoardConfigProvider>
  );
}
