import { useLayoutEffect, useEffect, useRef, useState } from 'react';
import type { Column } from '../api/types';

interface CardContextMenuProps {
  columns: Column[];
  currentColumn: string;
  hasCustomFields: boolean;
  x: number;
  y: number;
  onRename: () => void;
  onChangeFields: () => void;
  onMove: (columnName: string) => void;
  onClose: () => void;
}

export default function CardContextMenu({ columns, currentColumn, hasCustomFields, x, y, onRename, onChangeFields, onMove, onClose }: CardContextMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);
  const [positioned, setPositioned] = useState(false);

  // Adjust position to keep menu within viewport (useLayoutEffect to avoid flash)
  useLayoutEffect(() => {
    if (!menuRef.current) return;
    const rect = menuRef.current.getBoundingClientRect();
    const el = menuRef.current;

    if (rect.bottom > window.innerHeight) {
      el.style.top = `${Math.max(0, y - rect.height)}px`;
    }
    if (rect.right > window.innerWidth) {
      el.style.left = `${Math.max(0, x - rect.width)}px`;
    }
    setPositioned(true);
  }, [x, y]);

  // Close on Escape or click outside
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    // Delay click listener to avoid closing from the right-click event itself
    const timeoutId = setTimeout(() => {
      document.addEventListener('mousedown', handleClickOutside);
    }, 0);

    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      clearTimeout(timeoutId);
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [onClose]);

  const actionBtnClass = 'w-full px-3 py-1.5 text-left text-sm flex items-center gap-2 text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600';

  return (
    <div
      ref={menuRef}
      onContextMenu={(e) => e.preventDefault()}
      style={{ top: y, left: x, visibility: positioned ? 'visible' : 'hidden' }}
      className="fixed z-50 w-44 bg-white dark:bg-gray-700 rounded-lg shadow-lg border border-gray-200 dark:border-gray-600 py-1"
    >
      {/* Rename */}
      <button onClick={onRename} className={actionBtnClass}>
        <svg className="w-4 h-4 flex-shrink-0 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
        </svg>
        <span>Rename</span>
      </button>

      {/* Change Fields */}
      {hasCustomFields && (
        <button onClick={onChangeFields} className={actionBtnClass}>
          <svg className="w-4 h-4 flex-shrink-0 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
          </svg>
          <span>Change Fields</span>
        </button>
      )}

      {/* Separator */}
      <div className="my-1 border-t border-gray-200 dark:border-gray-600" />

      {/* Move to */}
      <div className="px-3 py-1.5 text-xs text-gray-400 dark:text-gray-500 font-medium uppercase tracking-wide">
        Move to
      </div>
      {columns.map((col) => {
        const isCurrent = col.name === currentColumn;
        return (
          <button
            key={col.name}
            onClick={() => !isCurrent && onMove(col.name)}
            disabled={isCurrent}
            className={`w-full px-3 py-1.5 text-left text-sm flex items-center gap-2 ${
              isCurrent
                ? 'text-gray-400 dark:text-gray-500 cursor-default'
                : 'text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600'
            }`}
          >
            <span
              className="w-2.5 h-2.5 rounded-full flex-shrink-0"
              style={{ backgroundColor: col.color }}
            />
            <span className="truncate">{col.name}</span>
            {isCurrent && (
              <span className="ml-auto text-xs text-gray-400 dark:text-gray-500">current</span>
            )}
          </button>
        );
      })}
    </div>
  );
}
