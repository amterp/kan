import { useEffect, useRef } from 'react';
import type { OmnibarMode } from '../hooks/useOmnibar';
import type { BoardEntry, SkippedProject } from '../api/types';

export type NavigationDirection = 'up' | 'down' | 'left' | 'right';

interface BoardsListProps {
  boards: BoardEntry[];
  highlightedIndex: number;
  currentProjectPath: string;
  currentBoardName: string | null;
  skipped: SkippedProject[];
  loading: boolean;
  error: string | null;
  displayLabel: (entry: BoardEntry) => string;
  onSelect: (index: number) => void;
}

function BoardsList({
  boards,
  highlightedIndex,
  currentProjectPath,
  currentBoardName,
  skipped,
  loading,
  error,
  displayLabel,
  onSelect,
}: BoardsListProps) {
  const listRef = useRef<HTMLDivElement>(null);
  const highlightedRef = useRef<HTMLDivElement>(null);

  // Keep highlighted item in view
  useEffect(() => {
    if (highlightedRef.current && listRef.current) {
      highlightedRef.current.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }
  }, [highlightedIndex]);

  if (loading) {
    return (
      <div className="py-3 px-4 text-sm text-gray-500 dark:text-gray-400">
        Loading projects...
      </div>
    );
  }

  if (error) {
    return (
      <div className="py-3 px-4 text-sm text-red-500">
        {error}
      </div>
    );
  }

  if (boards.length === 0) {
    return (
      <div className="py-3 px-4 text-sm text-gray-500 dark:text-gray-400">
        No matching boards found
      </div>
    );
  }

  return (
    <div ref={listRef} className="max-h-64 overflow-y-auto py-1">
      {boards.map((entry, idx) => {
        const isCurrent = entry.project_path === currentProjectPath && entry.board_name === currentBoardName;
        const isHighlighted = idx === highlightedIndex;
        return (
          <div
            key={`${entry.project_path}:${entry.board_name}`}
            ref={isHighlighted ? highlightedRef : undefined}
            onClick={() => onSelect(idx)}
            className={`px-4 py-2 cursor-pointer flex items-center gap-2 transition-colors ${
              isCurrent ? 'border-l-4 border-blue-500 dark:border-blue-400 pl-3' : 'border-l-4 border-transparent'
            } ${
              isHighlighted
                ? 'bg-blue-50 dark:bg-blue-900/30'
                : 'hover:bg-gray-50 dark:hover:bg-gray-700/50'
            }`}
          >
            <span className={`text-sm flex-1 ${
              isHighlighted
                ? 'text-blue-700 dark:text-blue-300 font-medium'
                : 'text-gray-700 dark:text-gray-300'
            }`}>
              {displayLabel(entry)}
            </span>
            <span className="text-xs text-gray-400 dark:text-gray-500 overflow-hidden text-ellipsis whitespace-nowrap max-w-md">
              {entry.project_path}
            </span>
          </div>
        );
      })}
      {skipped.length > 0 && (
        <div className="px-4 py-2 border-t border-gray-100 dark:border-gray-700">
          <span className="text-xs text-gray-400 dark:text-gray-500" title={skipped.map((s) => `${s.name}: ${s.reason}`).join('\n')}>
            {skipped.length} project{skipped.length !== 1 ? 's' : ''} unavailable
          </span>
        </div>
      )}
    </div>
  );
}

interface OmnibarProps {
  mode: OmnibarMode;
  query: string;
  matchCount: number;
  totalCount: number;
  hasHighlight: boolean;
  isModalOpen?: boolean;
  // Boards mode props
  boardEntries?: BoardEntry[];
  boardHighlightedIndex?: number;
  boardCurrentProjectPath?: string;
  boardCurrentBoardName?: string | null;
  boardSkipped?: SkippedProject[];
  boardLoading?: boolean;
  boardError?: string | null;
  boardDisplayLabel?: (entry: BoardEntry) => string;
  onQueryChange: (query: string) => void;
  onNavigate: (direction: NavigationDirection) => void;
  onSelect: () => void;
  onClose: () => void;
  onBoardSelect?: (index: number) => void;
}

export default function Omnibar({
  mode,
  query,
  matchCount,
  totalCount,
  hasHighlight,
  isModalOpen = false,
  boardEntries = [],
  boardHighlightedIndex = 0,
  boardCurrentProjectPath = '',
  boardCurrentBoardName = null,
  boardSkipped = [],
  boardLoading = false,
  boardError = null,
  boardDisplayLabel,
  onQueryChange,
  onNavigate,
  onSelect,
  onClose,
  onBoardSelect,
}: OmnibarProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const prevModalOpenRef = useRef(isModalOpen);

  // Auto-focus on mount
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Refocus input when modal closes
  useEffect(() => {
    if (prevModalOpenRef.current && !isModalOpen) {
      inputRef.current?.focus();
    }
    prevModalOpenRef.current = isModalOpen;
  }, [isModalOpen]);

  // Keyboard handling
  useEffect(() => {
    if (isModalOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'Escape':
          e.preventDefault();
          onClose();
          break;
        case 'ArrowUp':
          e.preventDefault();
          if (mode === 'boards') {
            onNavigate('up');
          } else {
            onNavigate('up');
          }
          break;
        case 'ArrowDown':
          e.preventDefault();
          if (mode === 'boards') {
            onNavigate('down');
          } else {
            onNavigate('down');
          }
          break;
        case 'ArrowLeft':
          if (mode === 'cards') {
            e.preventDefault();
            onNavigate('left');
          }
          // In boards mode, let cursor move naturally in input
          break;
        case 'ArrowRight':
          if (mode === 'cards') {
            e.preventDefault();
            onNavigate('right');
          }
          break;
        case 'Enter':
          e.preventDefault();
          onSelect();
          break;
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isModalOpen, mode, onClose, onNavigate, onSelect]);

  const placeholder = mode === 'boards' ? 'Switch board...' : 'Search cards...';
  const statusText = mode === 'boards'
    ? `${boardEntries.length} board${boardEntries.length !== 1 ? 's' : ''}`
    : `${matchCount} of ${totalCount} cards${hasHighlight && !isModalOpen ? ' · ↵ to open' : ''}`;

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center pb-16 pointer-events-none">
      <div className="absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-black/5 dark:from-black/20 to-transparent pointer-events-none" />
      <div className="pointer-events-auto flex flex-col items-center gap-2 min-w-96 max-w-lg w-full px-4">
        {/* Boards results list (renders above the bar) */}
        {mode === 'boards' && (
          <div className="w-full animate-omnibar-enter bg-white/95 backdrop-blur-sm dark:bg-gray-800/95 rounded-xl ring-1 ring-black/10 dark:ring-white/10 shadow-[0_8px_30px_rgb(0,0,0,0.25)] dark:shadow-[0_8px_30px_rgb(0,0,0,0.5)] overflow-hidden">
            <BoardsList
              boards={boardEntries}
              highlightedIndex={boardHighlightedIndex}
              currentProjectPath={boardCurrentProjectPath}
              currentBoardName={boardCurrentBoardName}
              skipped={boardSkipped}
              loading={boardLoading}
              error={boardError}
              displayLabel={boardDisplayLabel || ((e) => `${e.project_name} - ${e.board_name}`)}
              onSelect={(idx) => onBoardSelect?.(idx)}
            />
          </div>
        )}

        {/* Input bar */}
        <div className="w-full animate-omnibar-enter bg-white/95 backdrop-blur-sm dark:bg-gray-800/95 rounded-2xl ring-1 ring-black/10 dark:ring-white/10 px-5 py-4 flex items-center gap-3 shadow-[0_8px_30px_rgb(0,0,0,0.25)] dark:shadow-[0_8px_30px_rgb(0,0,0,0.5)]">
          {/* Search icon */}
          <svg
            className="w-5 h-5 text-gray-400 dark:text-gray-500 flex-shrink-0"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>

          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            placeholder={placeholder}
            className="flex-1 bg-transparent border-0 focus:outline-none text-lg text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500"
          />

          {/* Status text */}
          <span className="text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">
            {statusText}
          </span>

          {/* Close button */}
          <button
            onClick={onClose}
            className="p-1 text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 transition-colors"
            title="Close (Esc)"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}
