import type { Card, Label } from '../api/types';

interface CardModalProps {
  card: Card;
  labels: Label[];
  onClose: () => void;
}

export default function CardModal({ card, labels, onClose }: CardModalProps) {
  const cardLabels = card.labels
    ?.map((labelName) => labels.find((l) => l.name === labelName))
    .filter(Boolean) as Label[] | undefined;

  const formatDate = (millis: number) => {
    return new Date(millis).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between p-4 border-b border-gray-200">
          <div>
            <h2 className="text-xl font-semibold text-gray-900">{card.title}</h2>
            <p className="text-sm text-gray-500 font-mono mt-1">
              {card.alias} • {card.column}
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 p-1"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content */}
        <div className="p-4 space-y-4">
          {/* Labels */}
          {cardLabels && cardLabels.length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Labels</h3>
              <div className="flex flex-wrap gap-2">
                {cardLabels.map((label) => (
                  <span
                    key={label.name}
                    className="px-3 py-1 text-sm rounded-full text-white"
                    style={{ backgroundColor: label.color }}
                  >
                    {label.name}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Description */}
          {card.description && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Description</h3>
              <p className="text-gray-600 whitespace-pre-wrap">{card.description}</p>
            </div>
          )}

          {/* Metadata */}
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-gray-500">Created by</span>
              <p className="text-gray-900">{card.creator}</p>
            </div>
            <div>
              <span className="text-gray-500">Created</span>
              <p className="text-gray-900">{formatDate(card.created_at_millis)}</p>
            </div>
            <div>
              <span className="text-gray-500">Updated</span>
              <p className="text-gray-900">{formatDate(card.updated_at_millis)}</p>
            </div>
            <div>
              <span className="text-gray-500">ID</span>
              <p className="text-gray-900 font-mono">{card.id}</p>
            </div>
          </div>

          {/* Comments */}
          {card.comments && card.comments.length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">
                Comments ({card.comments.length})
              </h3>
              <div className="space-y-3">
                {card.comments.map((comment) => (
                  <div key={comment.id} className="bg-gray-50 rounded-lg p-3">
                    <div className="flex items-center justify-between mb-1">
                      <span className="font-medium text-gray-900 text-sm">
                        {comment.author}
                      </span>
                      <span className="text-xs text-gray-500">
                        {formatDate(comment.created_at_millis)}
                      </span>
                    </div>
                    <p className="text-gray-700 text-sm whitespace-pre-wrap">
                      {comment.body}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
