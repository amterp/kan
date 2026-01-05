import { useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Card as CardType, Label } from '../api/types';

interface CardProps {
  card: CardType;
  labels: Label[];
  isDragging?: boolean;
  isPlaceholder?: boolean;
  onClick?: () => void;
  onDelete?: () => void;
}

export default function Card({ card, labels, isDragging = false, isPlaceholder = false, onClick, onDelete }: CardProps) {
  const [showConfirm, setShowConfirm] = useState(false);
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging: isSortableDragging,
  } = useSortable({ id: card.id });

  // When this card is being dragged (shown as placeholder in originating column)
  const showAsPlaceholder = isPlaceholder || isSortableDragging;

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  const cardLabels = card.labels
    ?.map((labelName) => labels.find((l) => l.name === labelName))
    .filter(Boolean) as Label[] | undefined;

  const handleClick = () => {
    // Don't trigger click if we're dragging or confirming delete
    if (!isDragging && !isSortableDragging && !showConfirm && onClick) {
      onClick();
    }
  };

  const handleDeleteClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    setShowConfirm(true);
  };

  const handleConfirmDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDelete?.();
    setShowConfirm(false);
  };

  const handleCancelDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    setShowConfirm(false);
  };

  // Render as dashed placeholder when being dragged
  if (showAsPlaceholder) {
    return (
      <div
        ref={setNodeRef}
        style={style}
        {...attributes}
        {...listeners}
        className="bg-gray-100 border-2 border-dashed border-gray-300 rounded-lg p-3 min-h-[60px] opacity-50"
      />
    );
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      onClick={handleClick}
      className={`group relative bg-white rounded-lg p-3 shadow-sm cursor-pointer hover:shadow-md transition-shadow ${
        isDragging ? 'shadow-lg rotate-2' : ''
      }`}
    >
      {/* Delete confirmation overlay */}
      {showConfirm && (
        <div className="absolute inset-0 bg-white rounded-lg flex flex-col items-center justify-center gap-2 z-10">
          <p className="text-sm text-gray-700">Delete this card?</p>
          <div className="flex gap-2">
            <button
              onClick={handleConfirmDelete}
              className="px-3 py-1 text-sm bg-red-500 text-white rounded hover:bg-red-600"
            >
              Delete
            </button>
            <button
              onClick={handleCancelDelete}
              className="px-3 py-1 text-sm text-gray-600 hover:text-gray-800"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Trash icon - shown on hover */}
      {onDelete && !showConfirm && (
        <button
          onClick={handleDeleteClick}
          className="absolute top-1 right-1 p-1 text-gray-300 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity"
          title="Delete card"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>
      )}

      {/* Labels */}
      {cardLabels && cardLabels.length > 0 && (
        <div className="flex flex-wrap gap-1 mb-2">
          {cardLabels.map((label) => (
            <span
              key={label.name}
              className="px-2 py-0.5 text-xs rounded-full text-white"
              style={{ backgroundColor: label.color }}
            >
              {label.name}
            </span>
          ))}
        </div>
      )}

      {/* Title */}
      <h3 className="font-medium text-gray-900 text-sm">{card.title}</h3>

      {/* Footer */}
      <div className="flex items-center justify-between mt-2 text-xs text-gray-500">
        <span className="font-mono">{card.alias}</span>
        <div className="flex items-center gap-2">
          {card.description?.trim() && (
            <span title="Has description">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h7" />
              </svg>
            </span>
          )}
          {card.comments && card.comments.length > 0 && (
            <span className="flex items-center gap-1" title="Comments">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
              </svg>
              {card.comments.length}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
