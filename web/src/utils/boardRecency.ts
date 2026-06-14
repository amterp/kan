import type { BoardEntry } from '../api/types';

// Tracks when each board was last opened so both the home launcher and the
// omnibar board switcher can surface recently-used boards first. Keyed by
// "projectPath:boardName" and persisted in localStorage.

const RECENCY_KEY = 'kan-board-recency';
const MAX_RECENCY_ENTRIES = 50;

export interface RecencyMap {
  [key: string]: number; // "path:board" -> timestamp
}

export function recencyKey(entry: BoardEntry): string {
  return `${entry.project_path}:${entry.board_name}`;
}

export function loadRecency(): RecencyMap {
  try {
    const raw = localStorage.getItem(RECENCY_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

export function saveRecency(map: RecencyMap) {
  // Prune to the most recent entries to prevent unbounded growth.
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

// Stamp a board as just-opened. Called whenever a board is navigated to so the
// next launcher/switcher render orders it first.
export function recordRecency(entry: BoardEntry) {
  const map = loadRecency();
  map[recencyKey(entry)] = Date.now();
  saveRecency(map);
}

// Returns a new array sorted by recency (most recent first), falling back to
// the provided label for a stable alphabetical order among never-opened boards.
export function sortByRecency(
  entries: BoardEntry[],
  labelOf: (entry: BoardEntry) => string
): BoardEntry[] {
  const recency = loadRecency();
  return [...entries].sort((a, b) => {
    const aTime = recency[recencyKey(a)] || 0;
    const bTime = recency[recencyKey(b)] || 0;
    if (aTime !== bTime) return bTime - aTime;
    return labelOf(a).localeCompare(labelOf(b));
  });
}
