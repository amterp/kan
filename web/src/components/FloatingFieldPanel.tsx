import { useState, useRef, useEffect, useCallback } from 'react';
import type { BoardConfig } from '../api/types';
import CustomFieldsEditor from './CustomFieldsEditor';

interface FloatingFieldPanelProps {
  board: BoardConfig;
  values: Record<string, unknown>;
  onChange: (fieldName: string, value: unknown) => void;
  anchorEl: HTMLElement | null;
  onDismiss: () => void;
}

/**
 * Floating panel for editing custom fields after card creation.
 * Positions itself relative to the anchor element, flipping as needed
 * to stay within viewport.
 */
export default function FloatingFieldPanel({
  board,
  values,
  onChange,
  anchorEl,
  onDismiss,
}: FloatingFieldPanelProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null);

  // Don't render if no custom fields defined
  const hasCustomFields = board.custom_fields && Object.keys(board.custom_fields).length > 0;

  // Calculate position relative to anchor element
  const calculatePosition = useCallback(() => {
    if (!anchorEl || !panelRef.current) return;

    const anchorRect = anchorEl.getBoundingClientRect();
    const panelRect = panelRef.current.getBoundingClientRect();
    const viewport = {
      width: window.innerWidth,
      height: window.innerHeight,
    };

    const gap = 8;

    // Default: position to the right of anchor
    let left = anchorRect.right + gap;

    // Check if panel overflows right edge - flip to left side
    if (left + panelRect.width > viewport.width - gap) {
      left = anchorRect.left - panelRect.width - gap;
    }

    // Ensure it doesn't go off left edge
    if (left < gap) {
      left = gap;
    }

    // Default: align top of panel with top of anchor
    let top = anchorRect.top;

    // Check if panel overflows bottom - anchor from bottom instead
    if (top + panelRect.height > viewport.height - gap) {
      top = anchorRect.bottom - panelRect.height;
    }

    // Ensure it doesn't go off top edge
    if (top < gap) {
      top = gap;
    }

    setPosition({ top, left });
  }, [anchorEl]);

  // Calculate position after first render (need panel dimensions)
  useEffect(() => {
    if (!anchorEl || !hasCustomFields) return;

    // Use double RAF to ensure panel is rendered and measured correctly
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        calculatePosition();
      });
    });

    const handleResize = () => calculatePosition();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, [anchorEl, hasCustomFields, calculatePosition]);

  // Handle click outside
  useEffect(() => {
    if (!hasCustomFields) return;

    const handleClickOutside = (e: MouseEvent) => {
      if (
        panelRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        anchorEl &&
        !anchorEl.contains(e.target as Node)
      ) {
        onDismiss();
      }
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onDismiss();
      }
    };

    // Delay adding click listener to avoid immediate trigger
    const timeoutId = setTimeout(() => {
      document.addEventListener('mousedown', handleClickOutside);
    }, 0);
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      clearTimeout(timeoutId);
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [anchorEl, hasCustomFields, onDismiss]);

  // Don't render if no custom fields or no anchor
  if (!hasCustomFields || !anchorEl) {
    return null;
  }

  return (
    <div
      ref={panelRef}
      className={`fixed z-50 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 p-3 w-56 max-h-80 overflow-y-auto transition-opacity duration-150 ${
        position ? 'opacity-100' : 'opacity-0'
      }`}
      style={{
        top: position?.top ?? -9999,
        left: position?.left ?? -9999,
        // Hide off-screen until position is calculated
        visibility: position ? 'visible' : 'hidden',
      }}
    >
      <div className="flex items-center justify-between mb-2 pb-2 border-b border-gray-200 dark:border-gray-700">
        <span className="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">
          Fields
        </span>
        <button
          onClick={onDismiss}
          className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 p-0.5"
          title="Close"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <CustomFieldsEditor
        board={board}
        values={values}
        onChange={onChange}
        compact
      />
    </div>
  );
}
