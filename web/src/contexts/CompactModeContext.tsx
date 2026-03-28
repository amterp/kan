/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import type { ReactNode } from 'react';

interface CompactModeContextValue {
  globalDefault: boolean;
  boardOverrides: Record<string, boolean>;
  projectPath: string;
  setProjectPath: (path: string) => void;
  toggleForBoard: (boardName: string | null) => void;
}

const CompactModeContext = createContext<CompactModeContextValue | null>(null);

const GLOBAL_KEY = 'kan-compact-mode';
const BOARDS_KEY = 'kan-compact-mode-boards';
const PROJECT_PATH_KEY = 'kan-project-path';

function getStoredGlobalDefault(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return localStorage.getItem(GLOBAL_KEY) === 'true';
  } catch {
    return false;
  }
}

function getStoredBoardOverrides(): Record<string, boolean> {
  if (typeof window === 'undefined') return {};
  try {
    const raw = localStorage.getItem(BOARDS_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

function getStoredProjectPath(): string {
  if (typeof window === 'undefined') return '';
  try {
    return localStorage.getItem(PROJECT_PATH_KEY) || '';
  } catch {
    return '';
  }
}

function compactKey(projectPath: string, boardName: string): string {
  return projectPath ? `${projectPath}:${boardName}` : boardName;
}

export function CompactModeProvider({ children }: { children: ReactNode }) {
  // Read-only migration seed: the old global compact preference becomes the
  // fallback for boards that haven't been explicitly toggled yet.
  const [globalDefault] = useState<boolean>(getStoredGlobalDefault);
  const [boardOverrides, setBoardOverrides] = useState<Record<string, boolean>>(getStoredBoardOverrides);
  const [projectPath, setProjectPathState] = useState<string>(getStoredProjectPath);

  const setProjectPath = useCallback((path: string) => {
    setProjectPathState(path);
    try {
      localStorage.setItem(PROJECT_PATH_KEY, path);
    } catch {
      // Safari private browsing or storage-restricted environments
    }
  }, []);

  const toggleForBoard = useCallback((boardName: string | null) => {
    if (!boardName) return;
    setBoardOverrides((prev) => {
      const key = compactKey(projectPath, boardName);
      const currentValue = key in prev ? prev[key] : globalDefault;
      const next = { ...prev, [key]: !currentValue };
      try {
        localStorage.setItem(BOARDS_KEY, JSON.stringify(next));
      } catch {
        // Safari private browsing or storage-restricted environments
      }
      return next;
    });
  }, [globalDefault, projectPath]);

  return (
    <CompactModeContext.Provider value={{ globalDefault, boardOverrides, projectPath, setProjectPath, toggleForBoard }}>
      {children}
    </CompactModeContext.Provider>
  );
}

/**
 * Must be called from within a React Router context (inside BrowserRouter).
 * Uses useParams to read the current board name from the URL.
 */
export function useCompactMode() {
  const context = useContext(CompactModeContext);
  if (!context) {
    throw new Error('useCompactMode must be used within a CompactModeProvider');
  }

  const { boardName } = useParams<{ boardName: string }>();
  const { globalDefault, boardOverrides, projectPath, setProjectPath, toggleForBoard } = context;

  const key = boardName ? compactKey(projectPath, boardName) : null;

  const isCompact = key && key in boardOverrides
    ? boardOverrides[key]
    : globalDefault;

  const toggleCompact = useCallback(() => {
    toggleForBoard(boardName ?? null);
  }, [toggleForBoard, boardName]);

  return { isCompact, toggleCompact, setProjectPath };
}
