/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, useCallback } from 'react';
import type { ReactNode } from 'react';

interface SlimModeContextValue {
  isSlim: boolean;
  setSlim: (value: boolean) => void;
  toggleSlim: () => void;
}

const SlimModeContext = createContext<SlimModeContextValue | null>(null);

const STORAGE_KEY = 'kan-slim-mode';

function getInitialSlimMode(): boolean {
  // URL param takes priority over localStorage (enables sharing slim links)
  if (typeof window !== 'undefined') {
    try {
      const params = new URLSearchParams(window.location.search);
      if (params.get('slim') === 'true') return true;
    } catch {
      // Ignore URL parsing errors
    }
    try {
      return localStorage.getItem(STORAGE_KEY) === 'true';
    } catch {
      // Safari private browsing
    }
  }
  return false;
}

function persistSlim(value: boolean) {
  try {
    localStorage.setItem(STORAGE_KEY, String(value));
  } catch {
    // Safari private browsing or storage-restricted environments
  }
}

export function SlimModeProvider({ children }: { children: ReactNode }) {
  const [isSlim, setIsSlim] = useState<boolean>(getInitialSlimMode);

  const setSlim = useCallback((value: boolean) => {
    setIsSlim(value);
    persistSlim(value);
  }, []);

  const toggleSlim = useCallback(() => {
    setIsSlim((prev) => {
      const next = !prev;
      persistSlim(next);
      return next;
    });
  }, []);

  return (
    <SlimModeContext.Provider value={{ isSlim, setSlim, toggleSlim }}>
      {children}
    </SlimModeContext.Provider>
  );
}

/**
 * Returns global slim mode state. Unlike compact mode (which is per-board),
 * slim mode is a global display preference that persists across board switches.
 */
export function useSlimMode() {
  const context = useContext(SlimModeContext);
  if (!context) {
    throw new Error('useSlimMode must be used within a SlimModeProvider');
  }
  return context;
}
