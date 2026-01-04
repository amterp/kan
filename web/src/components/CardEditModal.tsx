import { useState, useRef, useEffect, useCallback } from 'react';
import type { Card, BoardConfig } from '../api/types';

interface CardEditModalProps {
  card: Card;
  board: BoardConfig;
  onSave: (updates: Partial<Card>) => Promise<void>;
  onClose: () => void;
}

// Module-level state to persist position/size across modal opens (ephemeral, lost on refresh)
let savedModalState = {
  position: { x: 0, y: 0 },
  size: { width: 900, height: 600 },
};

export default function CardEditModal({ card, board, onSave, onClose }: CardEditModalProps) {
  const [title, setTitle] = useState(card.title);
  const [description, setDescription] = useState(card.description || '');
  const [column, setColumn] = useState(card.column);
  const [selectedLabels, setSelectedLabels] = useState<string[]>(card.labels || []);
  const [saving, setSaving] = useState(false);

  // Drag state - initialize from saved state
  const [position, setPosition] = useState(savedModalState.position);
  const [isDragging, setIsDragging] = useState(false);
  const dragStartRef = useRef({ x: 0, y: 0, posX: 0, posY: 0 });

  // Resize state - initialize from saved state
  const [size, setSize] = useState(savedModalState.size);
  const [isResizing, setIsResizing] = useState(false);
  const resizeStartRef = useRef({ x: 0, y: 0, width: 0, height: 0 });

  // Track if we just finished resizing (to prevent backdrop click)
  const justFinishedInteraction = useRef(false);

  // Calculate hasChanges early so it can be used in effects
  const hasChanges =
    title !== card.title ||
    description !== (card.description || '') ||
    column !== card.column ||
    JSON.stringify(selectedLabels.sort()) !== JSON.stringify((card.labels || []).sort());

  // Save state when it changes
  useEffect(() => {
    savedModalState = { position, size };
  }, [position, size]);

  const handleGripMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
    dragStartRef.current = { x: e.clientX, y: e.clientY, posX: position.x, posY: position.y };
  }, [position]);

  const handleResizeStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsResizing(true);
    resizeStartRef.current = {
      x: e.clientX,
      y: e.clientY,
      width: size.width,
      height: size.height,
    };
  }, [size]);

  useEffect(() => {
    if (!isDragging && !isResizing) return;

    const handleMouseMove = (e: MouseEvent) => {
      if (isDragging) {
        const dx = e.clientX - dragStartRef.current.x;
        const dy = e.clientY - dragStartRef.current.y;
        setPosition({ x: dragStartRef.current.posX + dx, y: dragStartRef.current.posY + dy });
      } else if (isResizing) {
        // Symmetric resize: mouse movement changes size by 2x (grows from center)
        const dx = e.clientX - resizeStartRef.current.x;
        const dy = e.clientY - resizeStartRef.current.y;

        const newWidth = Math.max(500, resizeStartRef.current.width + dx * 2);
        const newHeight = Math.max(400, resizeStartRef.current.height + dy * 2);

        setSize({ width: newWidth, height: newHeight });
      }
    };

    const handleMouseUp = () => {
      if (isDragging || isResizing) {
        justFinishedInteraction.current = true;
        setTimeout(() => {
          justFinishedInteraction.current = false;
        }, 100);
      }
      setIsDragging(false);
      setIsResizing(false);
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isDragging, isResizing]);

  const formatDate = (millis: number) => {
    return new Date(millis).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const toggleLabel = (labelName: string) => {
    setSelectedLabels((prev) =>
      prev.includes(labelName)
        ? prev.filter((l) => l !== labelName)
        : [...prev, labelName]
    );
  };

  const handleSave = useCallback(async () => {
    if (!title.trim()) return;

    setSaving(true);
    try {
      await onSave({
        title: title.trim(),
        description: description.trim() || undefined,
        column,
        labels: selectedLabels.length > 0 ? selectedLabels : undefined,
      });
      onClose();
    } finally {
      setSaving(false);
    }
  }, [title, description, column, selectedLabels, onSave, onClose]);

  // Document-level keyboard handler so Cmd+Enter works without focus
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        if (hasChanges && title.trim()) {
          handleSave();
        } else {
          onClose();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose, hasChanges, title, handleSave]);

  const handleBackdropClick = () => {
    if (justFinishedInteraction.current) return;
    onClose();
  };

  // Button click handler - save if changes, close if no changes
  const handleButtonClick = () => {
    if (hasChanges) {
      handleSave();
    } else {
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 bg-black/30 flex items-center justify-center z-50"
      onClick={handleBackdropClick}
    >
      <div
        className="bg-white rounded-lg shadow-xl overflow-hidden flex flex-col relative"
        style={{
          width: size.width,
          height: size.height,
          maxWidth: '95vw',
          maxHeight: '95vh',
          transform: `translate(${position.x}px, ${position.y}px)`,
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Resize handle - bottom right corner */}
        <div
          className="absolute bottom-0 right-0 w-4 h-4 cursor-se-resize z-20 flex items-end justify-end"
          onMouseDown={handleResizeStart}
        >
          <svg
            className="w-3 h-3 text-gray-400"
            viewBox="0 0 24 24"
            fill="currentColor"
          >
            <circle cx="20" cy="20" r="2" />
            <circle cx="20" cy="12" r="2" />
            <circle cx="12" cy="20" r="2" />
          </svg>
        </div>

        {/* Header */}
        <div className="flex items-center p-4 border-b border-gray-200 bg-gray-50 flex-shrink-0">
          {/* Grip handle - larger hitbox for easier grabbing */}
          <div
            className="flex items-center justify-center w-10 h-12 mr-2 cursor-grab active:cursor-grabbing rounded hover:bg-gray-200"
            onMouseDown={handleGripMouseDown}
          >
            <div className="flex flex-col gap-0.5">
              <div className="flex gap-0.5">
                <div className="w-1 h-1 rounded-full bg-gray-400" />
                <div className="w-1 h-1 rounded-full bg-gray-400" />
              </div>
              <div className="flex gap-0.5">
                <div className="w-1 h-1 rounded-full bg-gray-400" />
                <div className="w-1 h-1 rounded-full bg-gray-400" />
              </div>
              <div className="flex gap-0.5">
                <div className="w-1 h-1 rounded-full bg-gray-400" />
                <div className="w-1 h-1 rounded-full bg-gray-400" />
              </div>
            </div>
          </div>

          <div className="flex-1 mr-4">
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="text-xl font-semibold text-gray-900 w-full border-0 border-b-2 border-transparent focus:border-blue-500 focus:outline-none bg-transparent"
              placeholder="Card title"
              autoFocus
            />
            <p className="text-sm text-gray-500 font-mono mt-1">
              {card.alias} • {column}
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

        {/* Content - two column layout */}
        <div className="flex-1 flex overflow-hidden">
          {/* Left side - Description and Comments */}
          <div className="flex-1 p-4 overflow-y-auto border-r border-gray-200">
            {/* Description */}
            <div className="mb-6">
              <label className="block text-sm font-medium text-gray-700 mb-2">Description</label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="w-full h-48 border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                placeholder="Add a description..."
              />
            </div>

            {/* Comments (read-only for now) */}
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">
                Comments {card.comments && card.comments.length > 0 && `(${card.comments.length})`}
              </h3>
              {card.comments && card.comments.length > 0 ? (
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
              ) : (
                <p className="text-sm text-gray-400 italic">No comments yet</p>
              )}
            </div>
          </div>

          {/* Right side - Details */}
          <div className="w-64 p-4 overflow-y-auto bg-gray-50 flex-shrink-0">
            {/* Column selector */}
            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-700 mb-1">Column</label>
              <select
                value={column}
                onChange={(e) => setColumn(e.target.value)}
                className="w-full border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
              >
                {board.columns.map((col) => (
                  <option key={col.name} value={col.name}>
                    {col.name}
                  </option>
                ))}
              </select>
            </div>

            {/* Labels */}
            {board.labels && board.labels.length > 0 && (
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-2">Labels</label>
                <div className="flex flex-wrap gap-2">
                  {board.labels.map((label) => {
                    const isSelected = selectedLabels.includes(label.name);
                    return (
                      <button
                        key={label.name}
                        type="button"
                        onClick={() => toggleLabel(label.name)}
                        className={`px-2 py-0.5 text-xs rounded-full transition-all ${
                          isSelected
                            ? 'text-white ring-2 ring-offset-1'
                            : 'text-white opacity-50 hover:opacity-75'
                        }`}
                        style={{
                          backgroundColor: label.color,
                          boxShadow: isSelected ? `0 0 0 2px white, 0 0 0 4px ${label.color}` : undefined,
                        }}
                      >
                        {label.name}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Metadata */}
            <div className="space-y-3 text-sm border-t border-gray-200 pt-4">
              <div>
                <span className="text-gray-500 block">Created by</span>
                <span className="text-gray-900">{card.creator}</span>
              </div>
              <div>
                <span className="text-gray-500 block">Created</span>
                <span className="text-gray-900">{formatDate(card.created_at_millis)}</span>
              </div>
              <div>
                <span className="text-gray-500 block">Updated</span>
                <span className="text-gray-900">{formatDate(card.updated_at_millis)}</span>
              </div>
              <div>
                <span className="text-gray-500 block">ID</span>
                <span className="text-gray-900 font-mono text-xs break-all">{card.id}</span>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between p-4 border-t border-gray-200 bg-gray-50 flex-shrink-0">
          <span className="text-xs text-gray-400">⌘↵ to {hasChanges ? 'save' : 'close'}</span>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-600 hover:text-gray-800"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleButtonClick}
              disabled={!title.trim() || saving}
              className="px-4 py-2 text-sm bg-blue-500 text-white rounded-md hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {saving ? 'Saving...' : hasChanges ? 'Save' : 'Close'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
