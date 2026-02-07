import { useMemo, useRef, useEffect, useState } from 'react';
import { useDroppable } from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy, useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Card, Column as ColumnType, BoardConfig, UpdateColumnInput } from '../api/types';
import CardComponent from './Card';
import ConfirmationModal from './ConfirmationModal';
import { useToast } from '../contexts/ToastContext';

// Transform user input to valid column name format (same as Board.tsx for createColumn)
function transformColumnName(input: string): string {
  return input.trim().toLowerCase().replace(/\s+/g, '-');
}

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
  onAddCard: (title: string, openModal: boolean, keepFormOpen?: boolean, showPanel?: boolean) => void;
  onCardClick: (card: Card) => void;
  onDeleteCard: (cardId: string) => void;
  onDeleteColumn?: (columnName: string) => Promise<unknown>;
  onUpdateColumn?: (columnName: string, updates: UpdateColumnInput) => Promise<unknown>;
  isEditingName: boolean;
  onStartEditName: () => void;
  onStopEditName: () => void;
  activeCard: Card | null;
  isOverColumn: boolean;
  overIndex: number | null;
  isDragging?: boolean;
  // Floating panel props
  onPanelHide?: () => void;
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
  onDeleteColumn,
  onUpdateColumn,
  isEditingName,
  onStartEditName,
  onStopEditName,
  activeCard,
  isOverColumn,
  overIndex,
  isDragging,
  onPanelHide,
}: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: column.name });
  const { showToast } = useToast();

  // Make the column header draggable for reordering
  const {
    attributes: sortableAttributes,
    listeners: sortableListeners,
    setNodeRef: setSortableNodeRef,
    transform,
    transition,
  } = useSortable({ id: column.name });

  const sortableStyle = {
    transform: CSS.Transform.toString(transform),
    transition,
  };
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const formRef = useRef<HTMLFormElement>(null);
  const [showMenu, setShowMenu] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Column name editing state (isEditingName is controlled by parent for stability)
  const [editName, setEditName] = useState(column.name);
  const [editError, setEditError] = useState<string | null>(null);
  const editInputRef = useRef<HTMLInputElement>(null);

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

  // Click outside to close column menu
  useEffect(() => {
    if (!showMenu) return;

    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowMenu(false);
      }
    };

    const timeoutId = setTimeout(() => {
      document.addEventListener('mousedown', handleClickOutside);
    }, 0);

    return () => {
      clearTimeout(timeoutId);
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [showMenu]);

  // Focus and select text when entering edit mode for column name
  // Note: editName is initialized via useState(column.name), so when component remounts
  // (e.g., after a failed rename reverts), it will have the correct value automatically
  useEffect(() => {
    if (isEditingName) {
      // Use setTimeout to ensure the input is rendered before focusing
      setTimeout(() => {
        editInputRef.current?.focus();
        editInputRef.current?.select();
      }, 0);
    }
  }, [isEditingName]);

  const handleDeleteColumnClick = () => {
    setShowMenu(false);
    setShowDeleteConfirm(true);
  };

  const handleConfirmDelete = async () => {
    if (!onDeleteColumn) return;

    try {
      await onDeleteColumn(column.name);
      setShowDeleteConfirm(false);
    } catch (err) {
      // Keep modal open on error so user knows something went wrong
      console.error('Failed to delete column:', err);
    }
  };

  // Column name editing handlers
  const handleStartEdit = (e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent column drag
    setEditName(column.name);
    setEditError(null);
    onStartEditName();
  };

  const handleSaveEdit = async () => {
    const newName = transformColumnName(editName);

    if (!newName) {
      setEditError('Column name cannot be empty');
      return;
    }

    if (newName === column.name) {
      onStopEditName();
      setEditError(null);
      return;
    }

    if (onUpdateColumn) {
      try {
        await onUpdateColumn(column.name, { name: newName });
        onStopEditName();
        setEditError(null);
      } catch (err) {
        // Parse error message for user-friendly display
        let message = 'Failed to rename column';
        if (err instanceof Error) {
          if (err.message.includes('already exists')) {
            message = `Column "${newName}" already exists`;
          } else {
            message = err.message;
          }
        }
        showToast('error', message);
        onStopEditName();
      }
    }
  };

  const handleCancelEdit = () => {
    onStopEditName();
    setEditName(column.name);
    setEditError(null);
  };

  const handleEditKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSaveEdit();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      handleCancelEdit();
    }
  };

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
      onPanelHide?.();
    } else if (e.key === 'Enter') {
      // Prevent newline in textarea
      e.preventDefault();
      if (e.metaKey || e.ctrlKey) {
        // Cmd+Enter or Ctrl+Enter - create and open full modal
        if (draftTitle.trim()) {
          onPanelHide?.();
          onAddCard(draftTitle.trim(), true);
          onDraftChange('');
        }
      } else if (e.shiftKey) {
        // Shift+Enter - create card, close form, show field panel anchored to card
        if (draftTitle.trim()) {
          onAddCard(draftTitle.trim(), false, false, true); // keepFormOpen=false, showPanel=true
          onDraftChange('');
        }
      } else {
        // Plain Enter - create card and continue (no panel)
        if (draftTitle.trim()) {
          onPanelHide?.();
          onAddCard(draftTitle.trim(), false, true);
          onDraftChange('');
          setTimeout(() => inputRef.current?.focus(), 0);
        }
      }
    }
  };

  const adjustTextareaHeight = (element: HTMLTextAreaElement) => {
    element.style.height = 'auto';
    element.style.height = `${element.scrollHeight}px`;
  };

  const cardCount = cards.length;
  const deleteMessage = cardCount > 0
    ? `This will permanently delete the column "${column.name}" and ${cardCount} card${cardCount === 1 ? '' : 's'}.`
    : `This will permanently delete the column "${column.name}".`;

  // Combine refs for both droppable (cards) and sortable (column reorder)
  const setRefs = (node: HTMLDivElement | null) => {
    setNodeRef(node);
    setSortableNodeRef(node);
  };

  return (
    <>
      <div
        ref={setRefs}
        style={{ ...sortableStyle, borderTopColor: column.color }}
        className={`flex-1 min-w-64 max-w-sm flex flex-col bg-gray-200 dark:bg-gray-800 rounded-lg max-h-full border-t-[3px] ${
          isOver ? 'ring-2 ring-blue-400' : ''
        } ${isDragging ? 'opacity-50' : ''}`}
      >
      {/* Column Header - draggable area */}
      <div
        className="flex items-center gap-2 px-3 py-2 border-b border-gray-300 dark:border-gray-600 cursor-grab active:cursor-grabbing"
        {...sortableAttributes}
        {...sortableListeners}
      >
        <button
          onClick={onStartAddCard}
          onPointerDown={(e) => e.stopPropagation()}
          className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 p-1 rounded hover:bg-gray-300 dark:hover:bg-gray-600"
          title="Add card"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
        {isEditingName ? (
          <div className="flex-1 min-w-0 relative">
            <input
              ref={editInputRef}
              type="text"
              value={editName}
              onChange={(e) => {
                setEditName(e.target.value);
                setEditError(null);
              }}
              onKeyDown={handleEditKeyDown}
              onBlur={handleSaveEdit}
              className={`w-full font-semibold text-gray-700 dark:text-gray-200 bg-transparent border-b-2 focus:outline-none px-0 py-0 ${
                editError ? 'border-red-500' : 'border-blue-500'
              }`}
            />
            {editError && (
              <p className="absolute text-xs text-red-500 mt-0.5 whitespace-nowrap">{editError}</p>
            )}
          </div>
        ) : (
          <>
            <h2
              className="font-semibold text-gray-700 dark:text-gray-200 truncate cursor-text hover:underline hover:decoration-dotted hover:decoration-gray-400 dark:hover:decoration-gray-500"
              onClick={handleStartEdit}
              onPointerDown={(e) => e.stopPropagation()}
              title="Click to rename"
            >
              {column.name}
            </h2>
            {column.description && (
              <span
                className="text-gray-400 dark:text-gray-500 flex-shrink-0 cursor-help"
                title={column.description}
                onPointerDown={(e) => e.stopPropagation()}
              >
                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </span>
            )}
            {/* Spacer to push card count and menu to the right - this area remains draggable */}
            <div className="flex-1" />
          </>
        )}
        <span className="text-sm text-gray-500 dark:text-gray-400 flex-shrink-0">{cards.length}</span>

        {/* Column Menu */}
        {(onDeleteColumn || onUpdateColumn) && (
          <div className="relative" ref={menuRef}>
            <button
              onClick={() => setShowMenu(!showMenu)}
              onPointerDown={(e) => e.stopPropagation()}
              className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 p-1 rounded hover:bg-gray-300 dark:hover:bg-gray-600"
              title="Column options"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z" />
              </svg>
            </button>

            {showMenu && (
              <div className="absolute right-0 mt-1 w-36 bg-white dark:bg-gray-700 rounded-lg shadow-lg border border-gray-200 dark:border-gray-600 py-1 z-50">
                {onDeleteColumn && column.name !== board.default_column && (
                  <button
                    onClick={handleDeleteColumnClick}
                    className="w-full px-3 py-2 text-left text-sm text-red-600 dark:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-600 flex items-center gap-2"
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                    Delete
                  </button>
                )}
                {onDeleteColumn && column.name === board.default_column && (
                  <div className="px-3 py-2 text-sm text-gray-400 dark:text-gray-500 italic">
                    Default column
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Cards */}
      <div className="flex-1 overflow-y-auto overflow-x-hidden p-2 space-y-2">
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
            <textarea
              ref={inputRef}
              value={draftTitle}
              onChange={(e) => {
                onDraftChange(e.target.value);
                adjustTextareaHeight(e.target);
              }}
              onKeyDown={handleKeyDown}
              placeholder="Enter card title..."
              rows={1}
              className="w-full px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-800 dark:text-white rounded focus:outline-none focus:ring-2 focus:ring-blue-500 placeholder:text-gray-400 dark:placeholder:text-gray-500 resize-none overflow-hidden"
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
                    onPanelHide?.();
                  }}
                  className="px-3 py-1 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600 rounded hover:shadow-sm transition-all"
                >
                  Cancel
                </button>
              </div>
              <span className="text-xs text-gray-400 dark:text-gray-500">⇧↵ fields · ⌘↵ modal</span>
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

      <ConfirmationModal
        isOpen={showDeleteConfirm}
        title="Delete Column"
        message={deleteMessage}
        confirmText="Delete"
        variant="danger"
        onConfirm={handleConfirmDelete}
        onCancel={() => setShowDeleteConfirm(false)}
      />
    </>
  );
}
