import { useState, useCallback, useMemo, useEffect } from 'react';
import { useTheme } from '../contexts/ThemeContext';
import type { Theme } from '../contexts/ThemeContext';
import { fuzzyMatch } from '../utils/fuzzyMatch';
import { THEME_PREFIX } from './omnibarConstants';

export interface ThemeOption {
  value: Theme;
  label: string;
}

const THEME_OPTIONS: ThemeOption[] = [
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
  { value: 'system', label: 'System' },
];

interface UseThemeSwitcherReturn {
  filteredOptions: ThemeOption[];
  highlightedIndex: number;
  currentTheme: Theme;
  setHighlightedIndex: (idx: number) => void;
  moveHighlight: (delta: number) => void;
  selectHighlighted: () => boolean;
  selectByIndex: (index: number) => boolean;
}

export function useThemeSwitcher(query: string, isActive: boolean): UseThemeSwitcherReturn {
  const { theme: currentTheme, setTheme } = useTheme();
  const [highlightedIndex, setHighlightedIndex] = useState(0);

  const filteredOptions = useMemo(() => {
    if (!isActive) return THEME_OPTIONS;
    const searchQuery = query.startsWith(THEME_PREFIX)
      ? query.slice(THEME_PREFIX.length).trim()
      : query.trim();
    if (!searchQuery) return THEME_OPTIONS;
    return THEME_OPTIONS.filter((opt) => fuzzyMatch(searchQuery, opt.label));
  }, [query, isActive]);

  // Reset highlight when filtered list changes
  useEffect(() => {
    setHighlightedIndex(0);
  }, [filteredOptions.length]);

  const moveHighlight = useCallback((delta: number) => {
    setHighlightedIndex((prev) => {
      const max = filteredOptions.length - 1;
      if (max < 0) return 0;
      const next = prev + delta;
      if (next < 0) return 0;
      if (next > max) return max;
      return next;
    });
  }, [filteredOptions.length]);

  const selectByIndex = useCallback((index: number): boolean => {
    const option = filteredOptions[index];
    if (!option) return false;
    setTheme(option.value);
    return true;
  }, [filteredOptions, setTheme]);

  const selectHighlighted = useCallback((): boolean => {
    return selectByIndex(highlightedIndex);
  }, [selectByIndex, highlightedIndex]);

  return {
    filteredOptions,
    highlightedIndex,
    currentTheme,
    setHighlightedIndex,
    moveHighlight,
    selectHighlighted,
    selectByIndex,
  };
}
