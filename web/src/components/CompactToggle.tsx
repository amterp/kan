import { useCompactMode } from '../contexts/CompactModeContext';

export default function CompactToggle() {
  const { isCompact, toggleCompact } = useCompactMode();

  return (
    <button
      onClick={toggleCompact}
      className={`p-2 rounded-md ${
        isCompact
          ? 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30 hover:bg-blue-100 dark:hover:bg-blue-900/50'
          : 'text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-700'
      }`}
      title={isCompact ? 'Compact view on (⌘C)' : 'Compact view (⌘C)'}
    >
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeWidth={2} d="M4 7h16M4 12h16M4 17h16" />
      </svg>
    </button>
  );
}
