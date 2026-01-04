interface HeaderProps {
  boards: string[];
  selectedBoard: string | null;
  onSelectBoard: (board: string) => void;
  onRefresh: () => void;
}

export default function Header({ boards, selectedBoard, onSelectBoard, onRefresh }: HeaderProps) {
  return (
    <header className="bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-between">
      <div className="flex items-center gap-4">
        <h1 className="text-xl font-bold text-gray-900">Kan</h1>
        {boards.length > 1 && (
          <select
            value={selectedBoard || ''}
            onChange={(e) => onSelectBoard(e.target.value)}
            className="border border-gray-300 rounded-md px-3 py-1.5 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
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
          <span className="text-gray-600">{boards[0]}</span>
        )}
      </div>
      <button
        onClick={onRefresh}
        className="text-gray-500 hover:text-gray-700 p-2 rounded-md hover:bg-gray-100"
        title="Refresh"
      >
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      </button>
    </header>
  );
}
