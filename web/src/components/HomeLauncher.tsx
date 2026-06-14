import { useEffect, useMemo, useState } from 'react';
import { listAllBoards } from '../api/projects';
import { sortByRecency } from '../utils/boardRecency';
import { resolveFavicon } from '../utils/favicon';
import type { BoardEntry, SkippedProject } from '../api/types';

interface HomeLauncherProps {
  onOpen: (entry: BoardEntry) => void;
}

function boardLabel(entry: BoardEntry): string {
  return `${entry.project_name}/${entry.board_name}`;
}

function BoardTile({ entry, onOpen }: { entry: BoardEntry; onOpen: (entry: BoardEntry) => void }) {
  const { background, glyph, isEmoji } = resolveFavicon(entry);
  return (
    <button
      onClick={() => onOpen(entry)}
      className="flex items-center gap-3 text-left bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-3 hover:border-blue-400 dark:hover:border-blue-500 hover:shadow-sm transition focus:outline-none focus:ring-2 focus:ring-blue-500"
    >
      <span
        className={`flex-shrink-0 w-10 h-10 rounded-md flex items-center justify-center text-white font-semibold ${isEmoji ? 'text-xl' : 'text-lg'}`}
        style={{ backgroundColor: background }}
        aria-hidden
      >
        {glyph}
      </span>
      <span className="min-w-0">
        <span className="block font-medium text-gray-900 dark:text-white truncate">
          {entry.board_name}
        </span>
        <span className="block text-xs text-gray-500 dark:text-gray-400 truncate">
          {entry.project_name}
        </span>
      </span>
    </button>
  );
}

function Section({
  title,
  entries,
  onOpen,
}: {
  title: string;
  entries: BoardEntry[];
  onOpen: (entry: BoardEntry) => void;
}) {
  if (entries.length === 0) return null;
  return (
    <section className="mb-8">
      <h2 className="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-3">
        {title}
      </h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        {entries.map((entry) => (
          <BoardTile key={`${entry.project_path}:${entry.board_name}`} entry={entry} onOpen={onOpen} />
        ))}
      </div>
    </section>
  );
}

export default function HomeLauncher({ onOpen }: HomeLauncherProps) {
  const [boards, setBoards] = useState<BoardEntry[]>([]);
  const [currentProjectPath, setCurrentProjectPath] = useState('');
  const [skipped, setSkipped] = useState<SkippedProject[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listAllBoards()
      .then((resp) => {
        if (cancelled) return;
        setBoards(resp.boards);
        setCurrentProjectPath(resp.current_project_path);
        setSkipped(resp.skipped || []);
        setError(null);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : 'Failed to load boards');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const { currentBoards, otherBoards, allBoards, currentProjectName } = useMemo(() => {
    const current = boards.filter((b) => b.project_path === currentProjectPath);
    const others = boards.filter((b) => b.project_path !== currentProjectPath);
    return {
      currentBoards: sortByRecency(current, boardLabel),
      otherBoards: sortByRecency(others, boardLabel),
      allBoards: sortByRecency(boards, boardLabel),
      currentProjectName: current[0]?.project_name || 'This project',
    };
  }, [boards, currentProjectPath]);

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center">
        <p className="text-gray-500 dark:text-gray-400">Loading boards...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="h-full flex items-center justify-center">
        <p className="text-red-500">{error}</p>
      </div>
    );
  }

  if (boards.length === 0) {
    return (
      <div className="h-full flex items-center justify-center">
        <p className="text-gray-500 dark:text-gray-400">No boards found.</p>
      </div>
    );
  }

  // Surface the current project's boards as their own section only when it has
  // several; otherwise a single recency-ordered grid reads cleaner.
  const splitCurrentProject = currentBoards.length >= 2;

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto px-6 py-8">
        <div className="flex items-baseline justify-between mb-6">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Boards</h1>
          <span className="text-xs text-gray-400 dark:text-gray-500 hidden sm:block">
            Press <kbd className="font-sans">⌘P</kbd> to search
          </span>
        </div>

        {splitCurrentProject ? (
          <>
            <Section title={currentProjectName} entries={currentBoards} onOpen={onOpen} />
            <Section title="Other boards" entries={otherBoards} onOpen={onOpen} />
          </>
        ) : (
          <Section title="All boards" entries={allBoards} onOpen={onOpen} />
        )}

        {skipped.length > 0 && (
          <p
            className="text-xs text-gray-400 dark:text-gray-500 mt-2"
            title={skipped.map((s) => `${s.name}: ${s.reason}`).join('\n')}
          >
            {skipped.length} project{skipped.length === 1 ? '' : 's'} skipped
          </p>
        )}
      </div>
    </div>
  );
}
