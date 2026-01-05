import { useTheme } from '../contexts/ThemeContext';
import type { Theme } from '../contexts/ThemeContext';

const themes: { value: Theme; icon: typeof SunIcon; label: string }[] = [
  { value: 'light', icon: SunIcon, label: 'Light mode' },
  { value: 'system', icon: ComputerIcon, label: 'System theme' },
  { value: 'dark', icon: MoonIcon, label: 'Dark mode' },
];

export default function ThemeToggle() {
  const { theme, setTheme } = useTheme();

  const selectedIndex = themes.findIndex((t) => t.value === theme);

  return (
    <div
      className="relative flex items-center bg-gray-200 dark:bg-gray-700 rounded-lg p-0.5"
      role="radiogroup"
      aria-label="Theme selection"
    >
      {/* Sliding indicator */}
      <div
        className="absolute h-7 w-7 bg-white dark:bg-gray-500 rounded-md shadow-sm transition-transform duration-200 ease-out"
        style={{ transform: `translateX(${selectedIndex * 28}px)` }}
      />

      {/* Theme buttons */}
      {themes.map(({ value, icon: Icon, label }) => (
        <button
          key={value}
          onClick={() => setTheme(value)}
          className={`relative z-10 w-7 h-7 flex items-center justify-center rounded-md transition-colors ${
            theme === value
              ? 'text-gray-900 dark:text-white'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
          }`}
          title={label}
          aria-label={label}
          aria-checked={theme === value}
          role="radio"
        >
          <Icon className="w-4 h-4" />
        </button>
      ))}
    </div>
  );
}

function SunIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
      />
    </svg>
  );
}

function MoonIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"
      />
    </svg>
  );
}

function ComputerIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}
