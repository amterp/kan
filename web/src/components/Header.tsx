import ThemeToggle from './ThemeToggle';

interface HeaderProps {
  boards: string[];
  selectedBoard: string | null;
  onSelectBoard: (board: string) => void;
  onRefresh: () => void;
  onNewCard?: () => void;
}

export default function Header({ boards, selectedBoard, onSelectBoard, onRefresh, onNewCard }: HeaderProps) {
  return (
    <header className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-2 flex items-center justify-between">
      <div className="flex items-center gap-4">
        <h1 className="text-4xl font-bold tracking-tight">
          <span className="bg-gradient-to-r from-blue-700 to-blue-500 dark:from-white dark:to-blue-300 bg-clip-text text-transparent">
            Kan
          </span>
        </h1>
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
        {onNewCard && (
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
        <a
          href="/docs"
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
        >
          Docs
        </a>
        <ThemeToggle />
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
