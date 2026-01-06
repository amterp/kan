import { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import type { Card, BoardConfig, CustomFieldSchema, UpdateCardInput } from '../api/types';
import { FIELD_TYPE_ENUM, FIELD_TYPE_TAGS, FIELD_TYPE_STRING, FIELD_TYPE_DATE } from '../api/types';
import MarkdownField from './MarkdownField';
import MarkdownView from './MarkdownView';

interface CardEditModalProps {
  card: Card;
  board: BoardConfig;
  onSave: (updates: UpdateCardInput) => Promise<void>;
  onDelete: () => void;
  onClose: () => void;
}

// Module-level state to persist position/size across modal opens (ephemeral, lost on refresh)
let savedModalState = {
  position: { x: 0, y: 0 },
  size: { width: 900, height: 600 },
};

// Helper to get current value for a custom field
function getFieldValue(card: Card, fieldName: string): unknown {
  return card[fieldName];
}

export default function CardEditModal({ card, board, onSave, onDelete, onClose }: CardEditModalProps) {
  const [title, setTitle] = useState(card.title);
  const [description, setDescription] = useState(card.description || '');
  const [column, setColumn] = useState(card.column);
  const [saving, setSaving] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Custom field states - initialized from card
  const [customFieldValues, setCustomFieldValues] = useState<Record<string, unknown>>(() => {
    const values: Record<string, unknown> = {};
    if (board.custom_fields) {
      for (const fieldName of Object.keys(board.custom_fields)) {
        const fieldValue = getFieldValue(card, fieldName);
        if (fieldValue !== undefined) {
          values[fieldName] = fieldValue;
        }
      }
    }
    return values;
  });

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
  const hasChanges = useMemo(() => {
    if (title !== card.title) return true;
    if (description !== (card.description || '')) return true;
    if (column !== card.column) return true;

    // Check custom fields
    if (board.custom_fields) {
      for (const fieldName of Object.keys(board.custom_fields)) {
        const originalValue = getFieldValue(card, fieldName);
        const currentValue = customFieldValues[fieldName];

        if (board.custom_fields[fieldName].type === 'tags') {
          const orig = Array.isArray(originalValue) ? originalValue : [];
          const curr = Array.isArray(currentValue) ? currentValue : [];
          if (JSON.stringify(orig.sort()) !== JSON.stringify(curr.sort())) return true;
        } else {
          if (originalValue !== currentValue) return true;
        }
      }
    }
    return false;
  }, [title, description, column, customFieldValues, card, board.custom_fields]);

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

  const toggleTagValue = (fieldName: string, value: string) => {
    setCustomFieldValues((prev) => {
      const current = Array.isArray(prev[fieldName]) ? prev[fieldName] as string[] : [];
      const newValues = current.includes(value)
        ? current.filter((v) => v !== value)
        : [...current, value];
      return { ...prev, [fieldName]: newValues };
    });
  };

  const setEnumValue = (fieldName: string, value: string) => {
    setCustomFieldValues((prev) => ({ ...prev, [fieldName]: value }));
  };

  const handleSave = useCallback(async () => {
    if (!title.trim()) return;

    setSaving(true);
    try {
      const updates: UpdateCardInput = {
        title: title.trim(),
        description: description.trim() || undefined,
        column,
      };

      // Build custom_fields for update
      if (board.custom_fields && Object.keys(board.custom_fields).length > 0) {
        const customFields: Record<string, unknown> = {};
        for (const [fieldName, schema] of Object.entries(board.custom_fields)) {
          const value = customFieldValues[fieldName];
          if (value !== undefined) {
            if (schema.type === 'tags' && Array.isArray(value)) {
              // Send tags as comma-separated string for API
              customFields[fieldName] = (value as string[]).join(',');
            } else {
              customFields[fieldName] = value;
            }
          }
        }
        if (Object.keys(customFields).length > 0) {
          updates.custom_fields = customFields;
        }
      }

      await onSave(updates);
      onClose();
    } finally {
      setSaving(false);
    }
  }, [title, description, column, customFieldValues, board.custom_fields, onSave, onClose]);

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

  // Render a custom field editor based on its type
  const renderCustomFieldEditor = (fieldName: string, schema: CustomFieldSchema) => {
    const currentValue = customFieldValues[fieldName];

    switch (schema.type) {
      case FIELD_TYPE_ENUM:
        return (
          <div className="mb-4" key={fieldName}>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">{fieldName}</label>
            <select
              value={(currentValue as string) || ''}
              onChange={(e) => setEnumValue(fieldName, e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white dark:bg-gray-700 dark:text-white"
            >
              <option value="">None</option>
              {schema.options?.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.value}
                </option>
              ))}
            </select>
          </div>
        );

      case FIELD_TYPE_TAGS: {
        const selectedTags = Array.isArray(currentValue) ? currentValue as string[] : [];
        return (
          <div className="mb-4" key={fieldName}>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2 capitalize">{fieldName}</label>
            <div className="flex flex-wrap gap-2">
              {schema.options?.map((opt) => {
                const isSelected = selectedTags.includes(opt.value);
                return (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => toggleTagValue(fieldName, opt.value)}
                    className={`px-2 py-0.5 text-xs rounded-full transition-all ${
                      isSelected
                        ? 'text-white ring-2 ring-offset-1'
                        : 'text-white opacity-50 hover:opacity-75'
                    }`}
                    style={{
                      backgroundColor: opt.color || '#6b7280',
                      boxShadow: isSelected ? `0 0 0 2px white, 0 0 0 4px ${opt.color || '#6b7280'}` : undefined,
                    }}
                  >
                    {opt.value}
                  </button>
                );
              })}
            </div>
          </div>
        );
      }

      case FIELD_TYPE_STRING:
        return (
          <div className="mb-4" key={fieldName}>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">{fieldName}</label>
            <input
              type="text"
              value={(currentValue as string) || ''}
              onChange={(e) => setCustomFieldValues((prev) => ({ ...prev, [fieldName]: e.target.value }))}
              className="w-full border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder={`Enter ${fieldName}...`}
            />
          </div>
        );

      case FIELD_TYPE_DATE:
        return (
          <div className="mb-4" key={fieldName}>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1 capitalize">{fieldName}</label>
            <input
              type="date"
              value={(currentValue as string) || ''}
              onChange={(e) => setCustomFieldValues((prev) => ({ ...prev, [fieldName]: e.target.value }))}
              className="w-full border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div
      className="fixed inset-0 bg-black/30 flex items-center justify-center z-50"
      onClick={handleBackdropClick}
    >
      <div
        className="bg-white dark:bg-gray-800 rounded-lg shadow-xl overflow-hidden flex flex-col relative"
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
            className="w-3 h-3 text-gray-400 dark:text-gray-500"
            viewBox="0 0 24 24"
            fill="currentColor"
          >
            <circle cx="20" cy="20" r="2" />
            <circle cx="20" cy="12" r="2" />
            <circle cx="12" cy="20" r="2" />
          </svg>
        </div>

        {/* Header */}
        <div className="flex items-center p-4 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 flex-shrink-0">
          {/* Grip handle - larger hitbox for easier grabbing */}
          <div
            className="flex items-center justify-center w-10 h-12 mr-2 cursor-grab active:cursor-grabbing rounded hover:bg-gray-200 dark:hover:bg-gray-700"
            onMouseDown={handleGripMouseDown}
          >
            <div className="flex flex-col gap-0.5">
              <div className="flex gap-0.5">
                <div className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500" />
                <div className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500" />
              </div>
              <div className="flex gap-0.5">
                <div className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500" />
                <div className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500" />
              </div>
              <div className="flex gap-0.5">
                <div className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500" />
                <div className="w-1 h-1 rounded-full bg-gray-400 dark:bg-gray-500" />
              </div>
            </div>
          </div>

          <div className="flex-1 mr-4">
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="text-xl font-semibold text-gray-900 dark:text-white w-full border-0 border-b-2 border-transparent focus:border-blue-500 focus:outline-none bg-transparent"
              placeholder="Card title"
              autoFocus
            />
            <p className="text-sm text-gray-500 dark:text-gray-400 font-mono mt-1">
              {card.alias} • {column}
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 p-1"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content - two column layout */}
        <div className="flex-1 flex overflow-hidden">
          {/* Left side - Description and Comments */}
          <div className="flex-1 p-4 overflow-y-auto border-r border-gray-200 dark:border-gray-700">
            {/* Description */}
            <div className="mb-6">
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Description</label>
              <MarkdownField
                value={description}
                onChange={setDescription}
                placeholder="Add a description..."
                minHeight="min-h-48"
              />
            </div>

            {/* Comments (read-only for now) */}
            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Comments {card.comments && card.comments.length > 0 && `(${card.comments.length})`}
              </h3>
              {card.comments && card.comments.length > 0 ? (
                <div className="space-y-3">
                  {card.comments.map((comment) => (
                    <div key={comment.id} className="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3">
                      <div className="flex items-center justify-between mb-1">
                        <span className="font-medium text-gray-900 dark:text-white text-sm">
                          {comment.author}
                        </span>
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          {formatDate(comment.created_at_millis)}
                        </span>
                      </div>
                      <MarkdownView content={comment.body} />
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-gray-400 dark:text-gray-500 italic">No comments yet</p>
              )}
            </div>
          </div>

          {/* Right side - Details */}
          <div className="w-64 p-4 overflow-y-auto bg-gray-50 dark:bg-gray-900 flex-shrink-0">
            {/* Column selector */}
            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Column</label>
              <select
                value={column}
                onChange={(e) => setColumn(e.target.value)}
                className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white dark:bg-gray-700 dark:text-white"
              >
                {board.columns.map((col) => (
                  <option key={col.name} value={col.name}>
                    {col.name}
                  </option>
                ))}
              </select>
            </div>

            {/* Custom fields */}
            {/* TODO: Field ordering is currently undefined (Go map → JSON → JS object).
                Consider adding explicit ordering config to boards in the future. */}
            {board.custom_fields && Object.entries(board.custom_fields).map(([fieldName, schema]) =>
              renderCustomFieldEditor(fieldName, schema)
            )}

            {/* Metadata */}
            <div className="space-y-3 text-sm border-t border-gray-200 dark:border-gray-700 pt-4">
              <div>
                <span className="text-gray-500 dark:text-gray-400 block">Created by</span>
                <span className="text-gray-900 dark:text-white">{card.creator}</span>
              </div>
              <div>
                <span className="text-gray-500 dark:text-gray-400 block">Created</span>
                <span className="text-gray-900 dark:text-white">{formatDate(card.created_at_millis)}</span>
              </div>
              <div>
                <span className="text-gray-500 dark:text-gray-400 block">Updated</span>
                <span className="text-gray-900 dark:text-white">{formatDate(card.updated_at_millis)}</span>
              </div>
              <div>
                <span className="text-gray-500 dark:text-gray-400 block">ID</span>
                <span className="text-gray-900 dark:text-white font-mono text-xs break-all">{card.id}</span>
              </div>
            </div>

            {/* Delete */}
            <div className="border-t border-gray-200 dark:border-gray-700 pt-4 mt-4">
              {showDeleteConfirm ? (
                <div className="space-y-2">
                  <p className="text-sm text-gray-700 dark:text-gray-200">Delete this card?</p>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={onDelete}
                      className="px-3 py-1 text-sm bg-red-500 text-white rounded hover:bg-red-600"
                    >
                      Delete
                    </button>
                    <button
                      type="button"
                      onClick={() => setShowDeleteConfirm(false)}
                      className="px-3 py-1 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              ) : (
                <button
                  type="button"
                  onClick={() => setShowDeleteConfirm(true)}
                  className="w-full flex items-center justify-center gap-2 px-3 py-2 text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md hover:bg-red-100 dark:hover:bg-red-900/30 hover:border-red-300 dark:hover:border-red-700 transition-colors"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                  Delete card
                </button>
              )}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between p-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 flex-shrink-0">
          <span className="text-xs text-gray-400 dark:text-gray-500">⌘↵ to {hasChanges ? 'save' : 'close'}</span>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
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
