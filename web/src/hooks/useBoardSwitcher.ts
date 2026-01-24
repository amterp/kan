import { useState, useEffect, useCallback, useMemo } from 'react';
import { listAllBoards, switchProject } from '../api/projects';
import { fuzzyMatch } from '../utils/fuzzyMatch';
import { BOARD_PREFIX } from './omnibarConstants';
import type { BoardEntry, SkippedProject } from '../api/types';

const RECENCY_KEY = 'kan-board-recency';
const MAX_RECENCY_ENTRIES = 50;

interface RecencyMap {
  [key: string]: number; // "path:board" -> timestamp
}

function loadRecency(): RecencyMap {
  try {
    const raw = localStorage.getItem(RECENCY_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

function saveRecency(map: RecencyMap) {
  // Prune to the most recent entries to prevent unbounded growth
  const entries = Object.entries(map);
  if (entries.length > MAX_RECENCY_ENTRIES) {
    entries.sort((a, b) => b[1] - a[1]); // Most recent first
    const pruned: RecencyMap = {};
    for (const [key, val] of entries.slice(0, MAX_RECENCY_ENTRIES)) {
      pruned[key] = val;
    }
    localStorage.setItem(RECENCY_KEY, JSON.stringify(pruned));
    return;
  }
  localStorage.setItem(RECENCY_KEY, JSON.stringify(map));
}

function recencyKey(entry: BoardEntry): string {
  return `${entry.project_path}:${entry.board_name}`;
}

function displayLabel(entry: BoardEntry, allBoards: BoardEntry[]): string {
  // If the project has exactly one board, omit the board name
  const projectBoards = allBoards.filter((b) => b.project_path === entry.project_path);
  if (projectBoards.length === 1) {
    return entry.project_name;
  }
  return `${entry.project_name} / ${entry.board_name}`;
}

interface UseBoardSwitcherReturn {
  boards: BoardEntry[];
  filteredBoards: BoardEntry[];
  highlightedIndex: number;
  currentProjectPath: string;
  skipped: SkippedProject[];
  loading: boolean;
  fetchError: string | null;
  switchError: string | null;
  setHighlightedIndex: (idx: number) => void;
  moveHighlight: (delta: number) => void;
  selectHighlighted: () => Promise<{ projectPath: string; boardName: string } | null>;
  displayLabel: (entry: BoardEntry) => string;
  refresh: () => void;
}

export function useBoardSwitcher(query: string, isActive: boolean): UseBoardSwitcherReturn {
  const [boards, setBoards] = useState<BoardEntry[]>([]);
  const [currentProjectPath, setCurrentProjectPath] = useState('');
  const [skipped, setSkipped] = useState<SkippedProject[]>([]);
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [switchError, setSwitchError] = useState<string | null>(null);

  const fetchBoards = useCallback(async () => {
    setLoading(true);
    setFetchError(null);
    try {
      const resp = await listAllBoards();
      setBoards(resp.boards);
      setCurrentProjectPath(resp.current_project_path);
      setSkipped(resp.skipped || []);
    } catch (e) {
      setFetchError(e instanceof Error ? e.message : 'Failed to load boards');
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch boards when the switcher becomes active
  useEffect(() => {
    if (isActive) {
      fetchBoards();
    }
  }, [isActive, fetchBoards]);

  // Filter and sort boards based on query
  const filteredBoards = useMemo(() => {
    const searchQuery = query.startsWith(BOARD_PREFIX)
      ? query.slice(BOARD_PREFIX.length).trim()
      : query.trim();

    let filtered = boards;
    if (searchQuery) {
      filtered = boards.filter((entry) => {
        const label = displayLabel(entry, boards);
        return fuzzyMatch(searchQuery, label);
      });
    }

    // Sort by recency, then alphabetical
    const recency = loadRecency();
    return filtered.sort((a, b) => {
      const aTime = recency[recencyKey(a)] || 0;
      const bTime = recency[recencyKey(b)] || 0;
      if (aTime !== bTime) return bTime - aTime; // More recent first
      const aLabel = displayLabel(a, boards);
      const bLabel = displayLabel(b, boards);
      return aLabel.localeCompare(bLabel);
    });
  }, [boards, query]);

  // Reset highlight when filtered results change
  useEffect(() => {
    setHighlightedIndex(0);
  }, [filteredBoards.length]);

  const moveHighlight = useCallback((delta: number) => {
    setHighlightedIndex((prev) => {
      const max = filteredBoards.length - 1;
      if (max < 0) return 0;
      const next = prev + delta;
      if (next < 0) return 0;
      if (next > max) return max;
      return next;
    });
  }, [filteredBoards.length]);

  const selectHighlighted = useCallback(async (): Promise<{ projectPath: string; boardName: string } | null> => {
    const entry = filteredBoards[highlightedIndex];
    if (!entry) return null;

    setSwitchError(null);
    try {
      const resp = await switchProject(entry.project_path);

      // Verify the selected board exists in the new project; fall back to first board
      const boardName = resp.boards.includes(entry.board_name)
        ? entry.board_name
        : resp.boards[0];

      // Update recency
      const recency = loadRecency();
      recency[recencyKey(entry)] = Date.now();
      saveRecency(recency);

      return { projectPath: entry.project_path, boardName };
    } catch (e) {
      setSwitchError(e instanceof Error ? e.message : 'Failed to switch project');
      return null;
    }
  }, [filteredBoards, highlightedIndex]);

  const getDisplayLabel = useCallback((entry: BoardEntry) => {
    return displayLabel(entry, boards);
  }, [boards]);

  return {
    boards,
    filteredBoards,
    highlightedIndex,
    currentProjectPath,
    skipped,
    loading,
    fetchError,
    switchError,
    setHighlightedIndex,
    moveHighlight,
    selectHighlighted,
    displayLabel: getDisplayLabel,
    refresh: fetchBoards,
  };
}
