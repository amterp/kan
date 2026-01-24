import { useState, useCallback } from 'react';
import { BOARD_PREFIX } from './omnibarConstants';

export type OmnibarMode = 'cards' | 'boards';

interface UseOmnibarReturn {
  isOpen: boolean;
  mode: OmnibarMode;
  query: string;
  highlightedCardId: string | null;
  open: (mode?: OmnibarMode) => void;
  close: () => void;
  setQuery: (query: string) => void;
  setHighlightedCardId: (id: string | null) => void;
}

export function useOmnibar(): UseOmnibarReturn {
  const [isOpen, setIsOpen] = useState(false);
  const [mode, setMode] = useState<OmnibarMode>('cards');
  const [query, setQueryState] = useState('');
  const [highlightedCardId, setHighlightedCardIdState] = useState<string | null>(null);

  const open = useCallback((openMode?: OmnibarMode) => {
    const m = openMode ?? 'cards';
    setMode(m);
    setIsOpen(true);
    if (m === 'boards') {
      setQueryState(BOARD_PREFIX);
    } else {
      setQueryState('');
    }
    setHighlightedCardIdState(null);
  }, []);

  const close = useCallback(() => {
    setIsOpen(false);
    setQueryState('');
    setHighlightedCardIdState(null);
    setMode('cards');
  }, []);

  const setQuery = useCallback((q: string) => {
    // Auto-switch mode based on /board prefix
    if (q.startsWith(BOARD_PREFIX) && mode !== 'boards') {
      setMode('boards');
    } else if (!q.startsWith(BOARD_PREFIX) && mode === 'boards') {
      // User deleted the prefix, switch back to cards mode
      setMode('cards');
    }
    setQueryState(q);
  }, [mode]);

  const setHighlightedCardId = useCallback((id: string | null) => {
    setHighlightedCardIdState(id);
  }, []);

  return { isOpen, mode, query, highlightedCardId, open, close, setQuery, setHighlightedCardId };
}
