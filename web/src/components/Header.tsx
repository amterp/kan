import { Link } from 'react-router-dom';
import ThemeToggle from './ThemeToggle';
import CompactToggle from './CompactToggle';
import SlimToggle from './SlimToggle';
import { useSlimMode } from '../contexts/SlimModeContext';
import type { BoardConfig } from '../api/types';

interface SyncStatus {
  connected: boolean;
  reconnecting: boolean;
  failed: boolean;
}

interface HeaderProps {
  boards: string[];
  selectedBoard: string | null;
  onSelectBoard: (board: string) => void;
  onRefresh: () => void;
  onNewCard?: () => void;
  syncStatus?: SyncStatus;
  // Custom-field sort control
  customFields?: BoardConfig['custom_fields'];
  sortField?: string;
  sortDescending?: boolean;
  onSortFieldChange?: (field: string) => void;
  onToggleSortDir?: () => void;
}

function SyncIndicator({ status, alwaysShowLabel }: { status: SyncStatus; alwaysShowLabel?: boolean }) {
  const labelClass = alwaysShowLabel ? '' : 'hidden sm:inline';
  if (status.connected) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-green-600 dark:text-green-400" title="Live sync active">
        <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
        <span className={labelClass}>Live</span>
      </div>
    );
  }
  if (status.reconnecting) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-yellow-600 dark:text-yellow-400" title="Reconnecting...">
        <span className="w-2 h-2 bg-yellow-500 rounded-full animate-pulse" />
        <span className={labelClass}>Reconnecting</span>
      </div>
    );
  }
  if (status.failed) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-red-600 dark:text-red-400" title="Live sync disconnected. Click refresh to update.">
        <span className="w-2 h-2 bg-red-500 rounded-full" />
        <span className={labelClass}>Disconnected</span>
      </div>
    );
  }
  return null;
}

export default function Header({
  boards,
  selectedBoard,
  onSelectBoard,
  onRefresh,
  onNewCard,
  syncStatus,
  customFields,
  sortField = '',
  sortDescending = false,
  onSortFieldChange,
  onToggleSortDir,
}: HeaderProps) {
  const { isSlim } = useSlimMode();
  const sortFieldNames = customFields ? Object.keys(customFields) : [];
  // A sort field carried over in the URL may not exist on this board; fall back
  // to "Manual order" so the dropdown never shows a blank/invalid selection.
  const effectiveSortField = sortFieldNames.includes(sortField) ? sortField : '';
  return (
    <header className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-2 flex items-center justify-between">
      <div className="flex items-center gap-4">
        <Link
          to="/"
          className="text-4xl font-bold text-gray-900 dark:text-white hover:opacity-80 transition-opacity"
          title="Home"
        >
          Kan
        </Link>
        {boards.length > 1 && (
          <select
            value={selectedBoard || ''}
            onChange={(e) => onSelectBoard(e.target.value)}
            className="border border-gray-300 dark:border-gray-600 rounded-md px-3 py-1.5 text-sm bg-white dark:bg-gray-700 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="" disabled>Select a board</option>
            {boards.map((board) => (
              <option key={board} value={board}>
                {board}
              </option>
            ))}
          </select>
        )}
        {boards.length === 1 && (
          <span className="text-gray-600 dark:text-gray-400">{boards[0]}</span>
        )}
        {!isSlim && onNewCard && (
          <button
            onClick={onNewCard}
            className="flex items-center gap-1 text-sm text-white bg-blue-500 hover:bg-blue-600 px-3 py-1.5 rounded-md"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            New Card
          </button>
        )}
      </div>
      <div className="flex items-center gap-2">
        {sortFieldNames.length > 0 && onSortFieldChange && (
          <div className="flex items-center gap-1" title="Sort cards within each column by a custom field (view only — does not change saved order)">
            <select
              value={effectiveSortField}
              onChange={(e) => onSortFieldChange(e.target.value)}
              aria-label="Sort cards by field"
              className="border border-gray-300 dark:border-gray-600 rounded-md px-2 py-1.5 text-sm bg-white dark:bg-gray-700 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="">Manual order</option>
              {sortFieldNames.map((name) => (
                <option key={name} value={name}>
                  Sort: {name}
                </option>
              ))}
            </select>
            {effectiveSortField && onToggleSortDir && (
              <button
                onClick={onToggleSortDir}
                title={sortDescending ? 'Descending — click for ascending' : 'Ascending — click for descending'}
                aria-label={sortDescending ? 'Sort descending' : 'Sort ascending'}
                className="text-gray-500 hover:text-gray-700 p-2 rounded-md hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-700"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  {sortDescending ? (
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
                  ) : (
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 10l7-7m0 0l7 7m-7-7v18" />
                  )}
                </svg>
              </button>
            )}
          </div>
        )}
        {syncStatus && <SyncIndicator status={syncStatus} alwaysShowLabel={isSlim} />}
        {!isSlim && (
          <a
            href="/docs"
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          >
            Docs
          </a>
        )}
        {!isSlim && <CompactToggle />}
        <SlimToggle />
        {!isSlim && <ThemeToggle />}
        <button
          onClick={onRefresh}
          className="text-gray-500 hover:text-gray-700 p-2 rounded-md hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-700"
          title="Refresh"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </div>
    </header>
  );
}
