import { useLocation, Link } from 'react-router-dom';
import MarkdownView from '../components/MarkdownView';

// Import docs as raw text
import indexDoc from '../docs/index.md?raw';
import editingDoc from '../docs/editing.md?raw';
import shortcutsDoc from '../docs/shortcuts.md?raw';
import cliDoc from '../docs/cli.md?raw';

const docs: Record<string, string> = {
  '': indexDoc,
  'index': indexDoc,
  'editing': editingDoc,
  'shortcuts': shortcutsDoc,
  'cli': cliDoc,
};

const navItems = [
  { path: '/docs', label: 'Home' },
  { path: '/docs/shortcuts', label: 'Keyboard Shortcuts' },
  { path: '/docs/editing', label: 'Editing Cards' },
  { path: '/docs/cli', label: 'CLI Reference' },
];

export default function DocsPage() {
  const location = useLocation();
  const isDocsOnly = import.meta.env.BASE_URL !== '/';

  // Extract doc name from path: /docs/editing -> editing, /docs -> ''
  const pathParts = location.pathname.split('/').filter(Boolean);
  const docName = pathParts.length > 1 ? pathParts.slice(1).join('/') : '';

  const content = docs[docName];
  const notFound = content === undefined;

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      {/* Header */}
      <header className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
        <div className="max-w-screen-2xl mx-auto flex items-center justify-between">
          <h1 className="text-xl font-semibold text-gray-900 dark:text-white">
            Kan Documentation
          </h1>
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

      <div className="max-w-screen-2xl mx-auto px-6 py-8 flex gap-8">
        {/* Sidebar */}
        <nav className="w-48 flex-shrink-0">
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
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-8">
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
