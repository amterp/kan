import { useEffect, useRef } from 'react';

export type NavigationDirection = 'up' | 'down' | 'left' | 'right';

interface OmnibarProps {
  query: string;
  matchCount: number;
  totalCount: number;
  hasHighlight: boolean;
  isModalOpen?: boolean;
  onQueryChange: (query: string) => void;
  onNavigate: (direction: NavigationDirection) => void;
  onSelect: () => void;
  onClose: () => void;
}

export default function Omnibar({
  query,
  matchCount,
  totalCount,
  hasHighlight,
  isModalOpen = false,
  onQueryChange,
  onNavigate,
  onSelect,
  onClose,
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
      // Modal just closed, refocus input
      inputRef.current?.focus();
    }
    prevModalOpenRef.current = isModalOpen;
  }, [isModalOpen]);

  // Keyboard handling: Escape, Arrow keys, Enter
  // Only handle when modal is not open
  useEffect(() => {
    if (isModalOpen) return; // Let modal handle its own keyboard events

    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'Escape':
          e.preventDefault();
          onClose();
          break;
        case 'ArrowUp':
          e.preventDefault();
          onNavigate('up');
          break;
        case 'ArrowDown':
          e.preventDefault();
          onNavigate('down');
          break;
        case 'ArrowLeft':
          e.preventDefault();
          onNavigate('left');
          break;
        case 'ArrowRight':
          e.preventDefault();
          onNavigate('right');
          break;
        case 'Enter':
          e.preventDefault();
          onSelect();
          break;
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isModalOpen, onClose, onNavigate, onSelect]);

  const handleBackdropClick = (e: React.MouseEvent) => {
    // Only close if clicking the backdrop itself, not the omnibar
    // And only if modal is not open
    if (e.target === e.currentTarget && !isModalOpen) {
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center pb-16"
      onClick={handleBackdropClick}
    >
      <div className="bg-white dark:bg-gray-800 rounded-2xl ring-1 ring-black/10 dark:ring-white/10 px-5 py-4 flex items-center gap-3 min-w-96 shadow-[0_8px_30px_rgb(0,0,0,0.25)] dark:shadow-[0_8px_30px_rgb(0,0,0,0.5)]">
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
          placeholder="Search cards..."
          className="flex-1 bg-transparent border-0 focus:outline-none text-lg text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500"
        />

        {/* Match count and navigation hint */}
        <span className="text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">
          {matchCount} of {totalCount} cards
          {hasHighlight && !isModalOpen && ' · ↵ to open'}
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
  );
}
