import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { loadRecency, saveRecency, recencyKey, recordRecency, sortByRecency } from './boardRecency';
import type { BoardEntry } from '../api/types';

// The tests run in the default node environment (no jsdom), so provide a tiny
// in-memory localStorage rather than pulling in a DOM environment.
function createLocalStorageMock() {
  let store: Record<string, string> = {};
  return {
    getItem: (k: string) => (k in store ? store[k] : null),
    setItem: (k: string, v: string) => {
      store[k] = String(v);
    },
    removeItem: (k: string) => {
      delete store[k];
    },
    clear: () => {
      store = {};
    },
    key: (i: number) => Object.keys(store)[i] ?? null,
    get length() {
      return Object.keys(store).length;
    },
  };
}

function entry(projectPath: string, boardName: string): BoardEntry {
  return { project_path: projectPath, board_name: boardName, project_name: projectPath };
}

describe('boardRecency', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', createLocalStorageMock());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it('round-trips the recency map through localStorage', () => {
    saveRecency({ 'a:main': 5 });
    expect(loadRecency()).toEqual({ 'a:main': 5 });
  });

  it('returns an empty map when nothing is stored', () => {
    expect(loadRecency()).toEqual({});
  });

  it('returns an empty map when stored JSON is corrupt', () => {
    localStorage.setItem('kan-board-recency', '{not json');
    expect(loadRecency()).toEqual({});
  });

  it('keys entries by project path and board name', () => {
    expect(recencyKey(entry('/repo/a', 'main'))).toBe('/repo/a:main');
  });

  it('prunes to the 50 most recent entries on save', () => {
    const map: Record<string, number> = {};
    for (let i = 0; i < 60; i++) {
      map[`p:${i}`] = i; // higher i = more recent
    }
    saveRecency(map);
    const loaded = loadRecency();
    expect(Object.keys(loaded)).toHaveLength(50);
    // Oldest (lowest timestamps) should be dropped.
    expect(loaded['p:0']).toBeUndefined();
    expect(loaded['p:59']).toBe(59);
  });

  it('stamps the opened board with the current time', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(1234));
    recordRecency(entry('/repo/a', 'main'));
    expect(loadRecency()['/repo/a:main']).toBe(1234);
  });

  describe('sortByRecency', () => {
    const label = (e: BoardEntry) => `${e.project_name}/${e.board_name}`;

    it('orders most-recently-opened first', () => {
      saveRecency({ '/a:main': 10, '/b:main': 30, '/c:main': 20 });
      const sorted = sortByRecency(
        [entry('/a', 'main'), entry('/b', 'main'), entry('/c', 'main')],
        label
      );
      expect(sorted.map((e) => e.project_path)).toEqual(['/b', '/c', '/a']);
    });

    it('falls back to alphabetical label for never-opened boards', () => {
      const sorted = sortByRecency([entry('/zebra', 'main'), entry('/apple', 'main')], label);
      expect(sorted.map((e) => e.project_path)).toEqual(['/apple', '/zebra']);
    });

    it('does not mutate the input array', () => {
      const input = [entry('/b', 'main'), entry('/a', 'main')];
      const copy = [...input];
      sortByRecency(input, label);
      expect(input).toEqual(copy);
    });
  });
});
