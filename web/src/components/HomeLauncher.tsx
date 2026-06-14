import { useEffect, useMemo, useState } from 'react';
import { listAllBoards } from '../api/projects';
import { sortByRecency } from '../utils/boardRecency';
import { resolveFavicon } from '../utils/favicon';
import type { BoardEntry, SkippedProject } from '../api/types';

interface HomeLauncherProps {
  onOpen: (entry: BoardEntry) => void;
}

// A board entry plus its display labels. The project is the unit you navigate
// to, so a single-board project leads with the project name and drops the
// (usually "main") board name; a project with several boards leads with the
// board name and keeps the project name as context to disambiguate.
interface LauncherTile {
  entry: BoardEntry;
  primary: string;
  secondary: string | null;
}

function boardLabel(entry: BoardEntry): string {
  return `${entry.project_name}/${entry.board_name}`;
}

// The project is what you scan for, so it leads. In a section already headed by
// the project name (the current-project section), that's redundant, so show the
// board name instead. Elsewhere the project name is the headline, with the board
// name beneath only when a project has several boards to tell apart.
function toTile(entry: BoardEntry, projectBoardCount: number, projectScoped: boolean): LauncherTile {
  if (projectScoped) {
    return { entry, primary: entry.board_name, secondary: null };
  }
  if (projectBoardCount <= 1) {
    return { entry, primary: entry.project_name, secondary: null };
  }
  return { entry, primary: entry.project_name, secondary: entry.board_name };
}

function BoardTile({ tile, onOpen }: { tile: LauncherTile; onOpen: (entry: BoardEntry) => void }) {
  const { entry, primary, secondary } = tile;
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
          {primary}
        </span>
        {secondary && (
          <span className="block text-sm text-gray-600 dark:text-gray-300 truncate">
            {secondary}
          </span>
        )}
      </span>
    </button>
  );
}

function Section({
  title,
  tiles,
  onOpen,
}: {
  title: string;
  tiles: LauncherTile[];
  onOpen: (entry: BoardEntry) => void;
}) {
  if (tiles.length === 0) return null;
  return (
    <section className="mb-8">
      <h2 className="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-3">
        {title}
      </h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        {tiles.map((tile) => (
          <BoardTile
            key={`${tile.entry.project_path}:${tile.entry.board_name}`}
            tile={tile}
            onOpen={onOpen}
          />
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

  const { currentTiles, otherTiles, allTiles, currentProjectName } = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const b of boards) {
      counts[b.project_path] = (counts[b.project_path] || 0) + 1;
    }
    const tilesOf = (entries: BoardEntry[], projectScoped: boolean) =>
      sortByRecency(entries, boardLabel).map((e) =>
        toTile(e, counts[e.project_path] || 1, projectScoped)
      );

    const current = boards.filter((b) => b.project_path === currentProjectPath);
    const others = boards.filter((b) => b.project_path !== currentProjectPath);
    return {
      // The current-project section is headed by the project name, so its tiles
      // show the board name; everywhere else the project name leads.
      currentTiles: tilesOf(current, true),
      otherTiles: tilesOf(others, false),
      allTiles: tilesOf(boards, false),
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
  const splitCurrentProject = currentTiles.length >= 2;

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
            <Section title={currentProjectName} tiles={currentTiles} onOpen={onOpen} />
            <Section title="Other boards" tiles={otherTiles} onOpen={onOpen} />
          </>
        ) : (
          <Section title="All boards" tiles={allTiles} onOpen={onOpen} />
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
