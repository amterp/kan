import { useState, useEffect } from 'react';
import { useLocation, Link } from 'react-router-dom';
import MarkdownView from '../components/MarkdownView';

// Import docs as raw text
import indexDoc from '../docs/index.md?raw';
import editingDoc from '../docs/editing.md?raw';
import shortcutsDoc from '../docs/shortcuts.md?raw';
import cliDoc from '../docs/cli.md?raw';
import linkRulesDoc from '../docs/link-rules.md?raw';

const docs: Record<string, string> = {
  '': indexDoc,
  'index': indexDoc,
  'editing': editingDoc,
  'shortcuts': shortcutsDoc,
  'cli': cliDoc,
  'link-rules': linkRulesDoc,
};

const navItems = [
  { path: '/docs', label: 'Home' },
  { path: '/docs/shortcuts', label: 'Keyboard Shortcuts' },
  { path: '/docs/editing', label: 'Editing Cards' },
  { path: '/docs/link-rules', label: 'Link Rules' },
  { path: '/docs/cli', label: 'CLI Reference' },
];

export default function DocsPage() {
  const location = useLocation();
  const isDocsOnly = import.meta.env.BASE_URL !== '/';
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  // Extract doc name from path: /docs/editing -> editing, /docs -> ''
  const pathParts = location.pathname.split('/').filter(Boolean);
  const docName = pathParts.length > 1 ? pathParts.slice(1).join('/') : '';

  const content = docs[docName];
  const notFound = content === undefined;

  // Close mobile menu on escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isMobileMenuOpen) {
        setIsMobileMenuOpen(false);
      }
    };
    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isMobileMenuOpen]);

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      {/* Header */}
      <header className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 md:px-6 py-4">
        <div className="max-w-screen-2xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-3">
            {/* Hamburger menu button - mobile only */}
            <button
              onClick={() => setIsMobileMenuOpen(true)}
              className="md:hidden p-1 -ml-1 text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white"
              aria-label="Open navigation menu"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
              </svg>
            </button>
            <h1 className="text-xl font-semibold text-gray-900 dark:text-white">
              Kan Documentation
            </h1>
          </div>
          {!isDocsOnly && (
            <Link
              to="/"
              className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
            >
              ← Back to board
            </Link>
          )}
        </div>
      </header>

      {/* Mobile drawer overlay */}
      {isMobileMenuOpen && (
        <div className="fixed inset-0 z-50 md:hidden">
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/50"
            onClick={() => setIsMobileMenuOpen(false)}
          />
          {/* Drawer */}
          <nav className="absolute left-0 top-0 h-full w-64 bg-white dark:bg-gray-800 shadow-xl animate-slide-in-left">
            <div className="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
              <span className="font-semibold text-gray-900 dark:text-white">Navigation</span>
              <button
                onClick={() => setIsMobileMenuOpen(false)}
                className="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
                aria-label="Close navigation menu"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <ul className="p-4 space-y-1">
              {navItems.map((item) => {
                const isActive = location.pathname === item.path ||
                  (item.path === '/docs' && location.pathname === '/docs/');
                return (
                  <li key={item.path}>
                    <Link
                      to={item.path}
                      onClick={() => setIsMobileMenuOpen(false)}
                      className={`block px-3 py-2 rounded-md text-sm ${
                        isActive
                          ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 font-medium'
                          : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
                      }`}
                    >
                      {item.label}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </nav>
        </div>
      )}

      <div className="max-w-screen-2xl mx-auto px-4 md:px-6 py-6 md:py-8 flex gap-4 md:gap-8">
        {/* Sidebar - hidden on mobile */}
        <nav className="hidden md:block w-48 flex-shrink-0">
          <ul className="space-y-1">
            {navItems.map((item) => {
              const isActive = location.pathname === item.path ||
                (item.path === '/docs' && location.pathname === '/docs/');
              return (
                <li key={item.path}>
                  <Link
                    to={item.path}
                    className={`block px-3 py-2 rounded-md text-sm ${
                      isActive
                        ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 font-medium'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
                    }`}
                  >
                    {item.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>

        {/* Content */}
        <main className="flex-1 min-w-0">
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4 md:p-8">
            {notFound ? (
              <div className="text-center py-12">
                <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-2">
                  Page not found
                </h2>
                <p className="text-gray-500 dark:text-gray-400 mb-4">
                  The documentation page you're looking for doesn't exist.
                </p>
                <Link
                  to="/docs"
                  className="text-blue-600 dark:text-blue-400 hover:underline"
                >
                  Go to documentation home
                </Link>
              </div>
            ) : (
              <MarkdownView content={content} />
            )}
          </div>
        </main>
      </div>
    </div>
  );
}
