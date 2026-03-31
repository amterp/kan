import { useLayoutEffect, useEffect, useRef, useState } from 'react';
import type { Column } from '../api/types';

interface CardContextMenuProps {
  columns: Column[];
  currentColumn: string;
  x: number;
  y: number;
  onMove: (columnName: string) => void;
  onClose: () => void;
}

export default function CardContextMenu({ columns, currentColumn, x, y, onMove, onClose }: CardContextMenuProps) {
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

  return (
    <div
      ref={menuRef}
      onContextMenu={(e) => e.preventDefault()}
      style={{ top: y, left: x, visibility: positioned ? 'visible' : 'hidden' }}
      className="fixed z-50 w-44 bg-white dark:bg-gray-700 rounded-lg shadow-lg border border-gray-200 dark:border-gray-600 py-1"
    >
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
