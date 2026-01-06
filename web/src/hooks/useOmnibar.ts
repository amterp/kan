import { useState, useCallback } from 'react';

interface UseOmnibarReturn {
  isOpen: boolean;
  query: string;
  highlightedCardId: string | null;
  open: () => void;
  close: () => void;
  setQuery: (query: string) => void;
  setHighlightedCardId: (id: string | null) => void;
}

export function useOmnibar(): UseOmnibarReturn {
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQueryState] = useState('');
  const [highlightedCardId, setHighlightedCardIdState] = useState<string | null>(null);

  const open = useCallback(() => {
    setIsOpen(true);
  }, []);

  // Close clears everything - query and highlight
  const close = useCallback(() => {
    setIsOpen(false);
    setQueryState('');
    setHighlightedCardIdState(null);
  }, []);

  const setQuery = useCallback((q: string) => {
    setQueryState(q);
  }, []);

  const setHighlightedCardId = useCallback((id: string | null) => {
    setHighlightedCardIdState(id);
  }, []);

  return { isOpen, query, highlightedCardId, open, close, setQuery, setHighlightedCardId };
}
