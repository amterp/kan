import { useState, useCallback, useMemo } from 'react';
import { SLASH_COMMANDS, type SlashCommand } from './omnibarConstants';

interface UseSlashCommandAutocompleteReturn {
  isActive: boolean;
  filteredCommands: SlashCommand[];
  highlightedIndex: number;
  moveHighlight: (delta: number) => void;
  setHighlightedIndex: (idx: number) => void;
}

export function useSlashCommandAutocomplete(query: string): UseSlashCommandAutocompleteReturn {
  // Store both the highlight index and the query that produced it. When the
  // query changes (user types), we reset to 0. This avoids useEffect + setState
  // which the linter flags as cascading renders.
  const [highlight, setHighlight] = useState<{ index: number; forQuery: string }>({ index: 0, forQuery: '' });
  const highlightedIndex = highlight.forQuery === query ? highlight.index : 0;

  // Active when the query starts with "/" and the user hasn't yet completed
  // a known command followed by a space (which would hand off to another mode).
  const isActive = useMemo(() => {
    if (!query.startsWith('/')) return false;
    const firstToken = query.split(' ')[0];
    const hasTrailingSpace = query.length > firstToken.length;
    // If the first token exactly matches a known command AND there's a trailing space,
    // the command has been "committed" (e.g. "/board " triggers boards mode).
    const exactMatch = SLASH_COMMANDS.some((c) => c.command === firstToken);
    if (exactMatch && hasTrailingSpace) return false;
    return true;
  }, [query]);

  const filteredCommands = useMemo(() => {
    if (!isActive) return [];
    const firstToken = query.split(' ')[0].toLowerCase();
    return SLASH_COMMANDS.filter((c) => c.command.startsWith(firstToken));
  }, [isActive, query]);

  const moveHighlight = useCallback((delta: number) => {
    setHighlight((prev) => {
      const len = filteredCommands.length;
      if (len === 0) return { index: 0, forQuery: query };
      const currentIdx = prev.forQuery === query ? prev.index : 0;
      return { index: ((currentIdx + delta) % len + len) % len, forQuery: query };
    });
  }, [filteredCommands.length, query]);

  const setHighlightedIndex = useCallback((idx: number) => {
    setHighlight({ index: idx, forQuery: query });
  }, [query]);

  return { isActive, filteredCommands, highlightedIndex, moveHighlight, setHighlightedIndex };
}
