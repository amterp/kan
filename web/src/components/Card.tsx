import { useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Card as CardType, Label } from '../api/types';
import CardModal from './CardModal';

interface CardProps {
  card: CardType;
  labels: Label[];
  isDragging?: boolean;
}

export default function Card({ card, labels, isDragging = false }: CardProps) {
  const [showModal, setShowModal] = useState(false);

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging: isSortableDragging,
  } = useSortable({ id: card.id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isSortableDragging ? 0.5 : 1,
  };

  const cardLabels = card.labels
    ?.map((labelName) => labels.find((l) => l.name === labelName))
    .filter(Boolean) as Label[] | undefined;

  return (
    <>
      <div
        ref={setNodeRef}
        style={style}
        {...attributes}
        {...listeners}
        onClick={() => !isDragging && setShowModal(true)}
        className={`bg-white rounded-lg p-3 shadow-sm cursor-pointer hover:shadow-md transition-shadow ${
          isDragging ? 'shadow-lg rotate-2' : ''
        }`}
      >
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
          {card.comments && card.comments.length > 0 && (
            <span className="flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
              </svg>
              {card.comments.length}
            </span>
          )}
        </div>
      </div>

      {showModal && (
        <CardModal card={card} labels={labels} onClose={() => setShowModal(false)} />
      )}
    </>
  );
}
