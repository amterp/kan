import { useSlimMode } from '../contexts/SlimModeContext';

export default function SlimToggle() {
  const { isSlim, toggleSlim } = useSlimMode();

  return (
    <button
      onClick={toggleSlim}
      className={`p-2 rounded-md ${
        isSlim
          ? 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30 hover:bg-blue-100 dark:hover:bg-blue-900/50'
          : 'text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-700'
      }`}
      title={isSlim ? 'Slim view on (⌘J)' : 'Slim view (⌘J)'}
    >
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <rect x="3" y="3" width="18" height="4" rx="1" strokeWidth={2} />
        <rect x="3" y="10" width="18" height="4" rx="1" strokeWidth={2} />
        <rect x="3" y="17" width="18" height="4" rx="1" strokeWidth={2} />
      </svg>
    </button>
  );
}
