import { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import type { Card, BoardConfig, CustomFieldSchema, UpdateCardInput, Comment } from '../api/types';
import { FIELD_TYPE_ENUM, FIELD_TYPE_TAGS, FIELD_TYPE_STRING, FIELD_TYPE_DATE } from '../api/types';
import { createComment, editComment, deleteComment } from '../api/cards';
import MarkdownField from './MarkdownField';
import MarkdownView from './MarkdownView';

interface CardEditModalProps {
  card: Card;
  board: BoardConfig;
  onSave: (updates: UpdateCardInput) => Promise<void>;
  onDelete: () => void;
  onClose: () => void;
}

// Calculate responsive initial size based on viewport
function getInitialModalSize() {
  return {
    width: Math.min(1200, window.innerWidth * 0.7),
    height: Math.min(900, window.innerHeight * 0.85),
  };
}

// Module-level state to persist position/size across modal opens (ephemeral, lost on refresh)
let savedModalState = {
  position: { x: 0, y: 0 },
  size: getInitialModalSize(),
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

  // Comment state
  const [comments, setComments] = useState<Comment[]>(card.comments || []);
  const [newCommentBody, setNewCommentBody] = useState('');
  const [editingCommentId, setEditingCommentId] = useState<string | null>(null);
  const [editingCommentBody, setEditingCommentBody] = useState('');
  const [commentSaving, setCommentSaving] = useState(false);

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

  // Track if mousedown started on backdrop (to prevent closing when drag-selecting text)
  const mouseDownOnBackdrop = useRef(false);

  // Ref for title textarea to adjust height on mount
  const titleRef = useRef<HTMLTextAreaElement>(null);

  const adjustTextareaHeight = useCallback((element: HTMLTextAreaElement) => {
    element.style.height = 'auto';
    element.style.height = `${element.scrollHeight}px`;
  }, []);

  // Adjust title textarea height on mount (in case of pre-existing long title)
  useEffect(() => {
    if (titleRef.current) {
      adjustTextareaHeight(titleRef.current);
    }
  }, [adjustTextareaHeight]);

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

  // Core save logic without closing the modal
  const performSave = useCallback(async () => {
    if (!title.trim()) return;

    setSaving(true);
    try {
      const updates: UpdateCardInput = {
        title: title.trim(),
        description: description.trim(),
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
    } finally {
      setSaving(false);
    }
  }, [title, description, column, customFieldValues, board.custom_fields, onSave]);

  const handleSave = useCallback(async () => {
    await performSave();
    onClose();
  }, [performSave, onClose]);

  // Close handler that saves changes first (used by Escape, backdrop click, X button)
  const handleCloseWithSave = useCallback(async () => {
    if (saving) return; // Prevent double-save if already saving
    if (hasChanges && title.trim()) {
      await performSave();
    }
    onClose();
  }, [saving, hasChanges, title, performSave, onClose]);

  // Document-level keyboard handler so Cmd+Enter works without focus
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        handleCloseWithSave();
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
  }, [onClose, hasChanges, title, handleSave, handleCloseWithSave]);

  const handleBackdropMouseDown = () => {
    mouseDownOnBackdrop.current = true;
  };

  const handleBackdropClick = () => {
    if (justFinishedInteraction.current) return;
    if (!mouseDownOnBackdrop.current) {
      mouseDownOnBackdrop.current = false;
      return;
    }
    mouseDownOnBackdrop.current = false;
    handleCloseWithSave();
  };

  // Button click handler - save if changes, close if no changes
  const handleButtonClick = () => {
    if (hasChanges) {
      handleSave();
    } else {
      onClose();
    }
  };

  // Comment handlers
  const handleAddComment = async () => {
    if (!newCommentBody.trim() || commentSaving) return;

    setCommentSaving(true);
    try {
      const comment = await createComment(board.name, card.id, newCommentBody.trim());
      setComments([...comments, comment]);
      setNewCommentBody('');
    } catch (error) {
      console.error('Failed to add comment:', error);
    } finally {
      setCommentSaving(false);
    }
  };

  const handleStartEditComment = (comment: Comment) => {
    setEditingCommentId(comment.id);
    setEditingCommentBody(comment.body);
  };

  const handleCancelEditComment = () => {
    setEditingCommentId(null);
    setEditingCommentBody('');
  };

  const handleSaveEditComment = async () => {
    if (!editingCommentId || !editingCommentBody.trim() || commentSaving) return;

    setCommentSaving(true);
    try {
      const updated = await editComment(board.name, card.id, editingCommentId, editingCommentBody.trim());
      setComments(comments.map(c => c.id === editingCommentId ? updated : c));
      setEditingCommentId(null);
      setEditingCommentBody('');
    } catch (error) {
      console.error('Failed to edit comment:', error);
    } finally {
      setCommentSaving(false);
    }
  };

  const handleDeleteComment = async (commentId: string) => {
    if (commentSaving) return;

    setCommentSaving(true);
    try {
      await deleteComment(board.name, card.id, commentId);
      setComments(comments.filter(c => c.id !== commentId));
    } catch (error) {
      console.error('Failed to delete comment:', error);
    } finally {
      setCommentSaving(false);
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
      onMouseDown={handleBackdropMouseDown}
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
        onMouseDown={(e) => e.stopPropagation()}
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
            <textarea
              ref={titleRef}
              value={title}
              onChange={(e) => {
                setTitle(e.target.value);
                adjustTextareaHeight(e.target);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  (e.target as HTMLTextAreaElement).blur();
                  if (hasChanges && title.trim()) {
                    performSave();
                  }
                }
              }}
              rows={1}
              className="text-xl font-semibold text-gray-900 dark:text-white w-full border-0 border-b-2 border-transparent focus:border-blue-500 focus:outline-none bg-transparent resize-none overflow-hidden"
              placeholder="Card title"
              autoFocus
            />
            <p className="text-sm text-gray-500 dark:text-gray-400 font-mono mt-1">
              {card.alias} • {column}
            </p>
          </div>
          <button
            onClick={handleCloseWithSave}
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
              <div className="flex items-center justify-between mb-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">Description</label>
                <a
                  href="/docs/editing"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs text-gray-400 hover:text-blue-500 dark:hover:text-blue-400"
                >
                  Formatting help
                </a>
              </div>
              <MarkdownField
                value={description}
                onChange={setDescription}
                placeholder="Add a description..."
                minHeight="min-h-48"
              />
            </div>

            {/* Comments */}
            <div>
              <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Comments {comments.length > 0 && `(${comments.length})`}
              </h3>

              {/* Existing comments */}
              {comments.length > 0 ? (
                <div className="space-y-3 mb-4">
                  {comments.map((comment) => (
                    <div key={comment.id} className="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-3 group">
                      <div className="flex items-center justify-between mb-1">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-gray-900 dark:text-white text-sm">
                            {comment.author}
                          </span>
                          <span className="text-xs text-gray-500 dark:text-gray-400">
                            {formatDate(comment.created_at_millis)}
                            {comment.updated_at_millis && comment.updated_at_millis > comment.created_at_millis && (
                              <> · Updated {formatDate(comment.updated_at_millis)}</>
                            )}
                          </span>
                        </div>
                        {editingCommentId !== comment.id && (
                          <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                            <button
                              type="button"
                              onClick={() => handleStartEditComment(comment)}
                              className="p-1 text-gray-400 hover:text-blue-500 dark:hover:text-blue-400"
                              title="Edit comment"
                            >
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                              </svg>
                            </button>
                            <button
                              type="button"
                              onClick={() => handleDeleteComment(comment.id)}
                              className="p-1 text-gray-400 hover:text-red-500 dark:hover:text-red-400"
                              title="Delete comment"
                            >
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                              </svg>
                            </button>
                          </div>
                        )}
                      </div>
                      {editingCommentId === comment.id ? (
                        <div>
                          <MarkdownField
                            value={editingCommentBody}
                            onChange={setEditingCommentBody}
                            placeholder="Edit comment..."
                            minHeight="min-h-20"
                            alwaysEditing
                            onSubmit={handleSaveEditComment}
                          />
                          <div className="flex gap-2 mt-2">
                            <button
                              type="button"
                              onClick={handleSaveEditComment}
                              disabled={commentSaving || !editingCommentBody.trim()}
                              className="px-3 py-1 text-sm bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50"
                            >
                              {commentSaving ? 'Saving...' : 'Save'}
                            </button>
                            <button
                              type="button"
                              onClick={handleCancelEditComment}
                              className="px-3 py-1 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
                            >
                              Cancel
                            </button>
                          </div>
                        </div>
                      ) : (
                        <MarkdownView content={comment.body} />
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-gray-400 dark:text-gray-500 italic mb-4">No comments yet</p>
              )}

              {/* New comment input */}
              <div className="border-t border-gray-200 dark:border-gray-700 pt-4">
                <MarkdownField
                  value={newCommentBody}
                  onChange={setNewCommentBody}
                  placeholder="Add a comment..."
                  minHeight="min-h-24"
                  alwaysEditing
                  onSubmit={handleAddComment}
                />
                <div className="flex justify-end mt-2">
                  <button
                    type="button"
                    onClick={handleAddComment}
                    disabled={commentSaving || !newCommentBody.trim()}
                    className="px-3 py-1 text-sm bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {commentSaving ? 'Adding...' : 'Add Comment'}
                  </button>
                </div>
              </div>
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
