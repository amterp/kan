import { useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Card as CardType, BoardConfig, CustomFieldOption } from '../api/types';
import { parseTextWithLinks } from '../utils/linkParser';

interface CardProps {
  card: CardType;
  board: BoardConfig;
  isDragging?: boolean;
  isPlaceholder?: boolean;
  isHighlighted?: boolean;
  onClick?: () => void;
  onDelete?: () => void;
}

// Helper to get option details for a field value
function getFieldOption(board: BoardConfig, fieldName: string, value: string): CustomFieldOption | undefined {
  const schema = board.custom_fields?.[fieldName];
  if (!schema?.options) return undefined;
  return schema.options.find(opt => opt.value === value);
}

// Helper to get array of values from a set field (enum-set or free-set)
function getSetValues(card: CardType, fieldName: string): string[] {
  const value = card[fieldName];
  if (!value) return [];
  if (Array.isArray(value)) return value as string[];
  if (typeof value === 'string') return [value];
  return [];
}

/**
 * Card component renders a kanban card with drag-and-drop support.
 *
 * NOTE: The data-card-id attribute is used by Board.tsx to find this element
 * when anchoring the FloatingFieldPanel after card creation. Don't remove it.
 */
export default function Card({ card, board, isDragging = false, isPlaceholder = false, isHighlighted = false, onClick, onDelete }: CardProps) {
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

  // Get type indicator field for the card
  const typeIndicator = board.card_display?.type_indicator;
  const typeValue = typeIndicator ? card[typeIndicator] as string : undefined;
  const typeOption = typeValue ? getFieldOption(board, typeIndicator!, typeValue) : undefined;

  // Get badge fields for the card
  const badgeFields = board.card_display?.badges || [];

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
        data-card-id={card.id}
        {...attributes}
        {...listeners}
        className="bg-gray-100 dark:bg-gray-600 border-2 border-dashed border-gray-300 dark:border-gray-500 rounded-lg p-3 min-h-[60px] opacity-50"
      />
    );
  }

  const cardStyle = {
    ...style,
    ...(typeOption?.color ? { borderLeftWidth: '3px', borderLeftStyle: 'solid' as const, borderLeftColor: typeOption.color } : {}),
  };

  return (
    <div
      ref={setNodeRef}
      style={cardStyle}
      data-card-id={card.id}
      {...attributes}
      {...listeners}
      onClick={handleClick}
      className={`group relative bg-white dark:bg-gray-700 rounded-lg p-3 shadow-sm border border-gray-100 dark:border-gray-600 cursor-pointer hover:shadow-md transition-shadow animate-card-enter ${
        isDragging ? 'shadow-lg rotate-2' : ''
      } ${isHighlighted ? 'ring-2 ring-blue-500 ring-offset-2 ring-offset-gray-200 dark:ring-offset-gray-800' : ''}`}
    >
      {/* Delete confirmation overlay */}
      {showConfirm && (
        <div className="absolute inset-0 bg-white dark:bg-gray-700 rounded-lg flex flex-col items-center justify-center gap-2 z-10">
          <p className="text-sm text-gray-700 dark:text-gray-200">Delete this card?</p>
          <div className="flex gap-2">
            <button
              onClick={handleConfirmDelete}
              className="px-3 py-1 text-sm bg-red-500 text-white rounded hover:bg-red-600"
            >
              Delete
            </button>
            <button
              onClick={handleCancelDelete}
              className="px-3 py-1 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
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
          className="absolute top-1 right-1 p-1 text-gray-300 dark:text-gray-500 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity"
          title="Delete card"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>
      )}

      {/* Badges row: type indicator (if any) + badge fields, all on one line above title */}
      {(() => {
        // Collect all badges: type indicator first, then badge field values
        const allBadges: { key: string; value: string; color: string }[] = [];

        // Add type indicator if configured and has value
        if (typeOption && typeValue) {
          allBadges.push({
            key: `type-${typeValue}`,
            value: typeValue,
            color: typeOption.color || '#6b7280',
          });
        }

        // Add badge field values
        for (const fieldName of badgeFields) {
          const values = getSetValues(card, fieldName);
          for (const value of values) {
            const option = getFieldOption(board, fieldName, value);
            allBadges.push({
              key: `${fieldName}-${value}`,
              value,
              color: option?.color || '#6b7280',
            });
          }
        }

        if (allBadges.length === 0) return null;

        return (
          <div className="flex flex-wrap gap-1 mb-2">
            {allBadges.map(badge => (
              <span
                key={badge.key}
                className="px-2 py-0.5 text-xs rounded-full text-white"
                style={{ backgroundColor: badge.color }}
              >
                {badge.value}
              </span>
            ))}
          </div>
        );
      })()}

      {/* Title */}
      <h3 className="font-medium text-gray-900 dark:text-white text-sm break-words">
        {parseTextWithLinks(card.title, board.link_rules).map((segment, i) =>
          segment.type === 'link' ? (
            <a
              key={i}
              href={segment.url}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              className="text-blue-600 dark:text-blue-400 hover:underline"
            >
              {segment.content}
            </a>
          ) : (
            <span key={i}>{segment.content}</span>
          )
        )}
      </h3>

      {/* Footer */}
      <div className="flex items-center justify-between mt-2 text-xs text-gray-500 dark:text-gray-400">
        <span className="font-mono">{card.alias}</span>
        <div className="flex items-center gap-2">
          {card.missing_wanted_fields && card.missing_wanted_fields.length > 0 && (
            <span className="relative group/warning">
              <svg className="w-4 h-4 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <span className="absolute bottom-full right-0 mb-1 px-2 py-1 text-xs text-white bg-gray-800 dark:bg-gray-900 rounded shadow-lg whitespace-nowrap opacity-0 group-hover/warning:opacity-100 transition-opacity pointer-events-none z-50">
                Missing: {card.missing_wanted_fields.map(f => f.name).join(', ')}
              </span>
            </span>
          )}
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
