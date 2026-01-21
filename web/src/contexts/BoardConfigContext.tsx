/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext } from 'react';
import type { ReactNode } from 'react';
import type { BoardConfig } from '../api/types';

interface BoardConfigContextValue {
  board: BoardConfig | null;
}

const BoardConfigContext = createContext<BoardConfigContextValue | null>(null);

interface BoardConfigProviderProps {
  board: BoardConfig;
  children: ReactNode;
}

/**
 * Provides board configuration to descendant components.
 *
 * This context makes board config (especially link_rules) available without
 * prop drilling through intermediate components. Components like MarkdownView
 * can access link_rules directly rather than having it passed through props.
 */
export function BoardConfigProvider({ board, children }: BoardConfigProviderProps) {
  return (
    <BoardConfigContext.Provider value={{ board }}>
      {children}
    </BoardConfigContext.Provider>
  );
}

/**
 * Returns the current board configuration.
 *
 * @throws Error if used outside of BoardConfigProvider
 */
export function useBoardConfig(): BoardConfig {
  const context = useContext(BoardConfigContext);
  if (!context || !context.board) {
    throw new Error('useBoardConfig must be used within a BoardConfigProvider');
  }
  return context.board;
}

/**
 * Returns the current board configuration, or null if not in a provider.
 *
 * Use this when the component might be rendered outside of a board context
 * (e.g., in documentation or standalone usage).
 */
export function useBoardConfigOptional(): BoardConfig | null {
  const context = useContext(BoardConfigContext);
  return context?.board ?? null;
}
