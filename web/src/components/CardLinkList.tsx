import type { CardLink } from '../api/types';

interface CardLinkListProps {
  label: string;
  links: CardLink[];
  onOpen: (ref: string) => void;
  disabled?: boolean;
}

export default function CardLinkList({ label, links, onOpen, disabled }: CardLinkListProps) {
  if (links.length === 0) return null;
  return (
    <section className="mb-4" aria-label={label}>
      <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{label}</h3>
      <ul className="space-y-2 text-sm">
        {links.map((link) => (
          <li key={`${link.project_path}:${link.ref}`}>
            {link.result === 'found' && link.card ? (
              <button type="button" disabled={disabled} onClick={() => onOpen(link.ref)}
                className="text-left text-blue-600 dark:text-blue-400 hover:underline disabled:opacity-50 break-words">
                {link.card.title}
                <span className="block text-xs text-gray-500 dark:text-gray-400">
                  {[link.project_name, link.board, link.card.column].filter(Boolean).join(' / ')}
                </span>
              </button>
            ) : (
              <span className="text-gray-500 dark:text-gray-400 break-all">
                {link.ref}<span className="block text-xs">{link.linked_name ? `${link.linked_name}: ` : ''}{link.reason || 'Unavailable blocker'}</span>
                {link.min_kan_version && <span className="block text-xs">Requires Kan {link.min_kan_version}</span>}
                {link.linked_remote && <span className="block text-xs">One place to get it: {link.linked_remote}</span>}
              </span>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
