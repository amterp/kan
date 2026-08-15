// @vitest-environment jsdom
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../api/client';
import type { BoardConfig, Card, FileChange } from '../api/types';
import { FileSyncProvider } from '../contexts/FileSyncContext';
import { ToastProvider } from '../contexts/ToastContext';
import { FakeWebSocket, installFakeWebSocket } from '../test/fakeWebSocket';
import { useBoard, useBoards } from './useBoards';

vi.mock('../api/boards', () => ({
  listBoards: vi.fn(),
  getBoard: vi.fn(),
  createColumn: vi.fn(),
  deleteColumn: vi.fn(),
  updateColumn: vi.fn(),
  reorderColumns: vi.fn(),
}));

vi.mock('../api/cards', () => ({
  listCards: vi.fn(),
  getCard: vi.fn(),
  moveCard: vi.fn(),
  createCard: vi.fn(),
  updateCard: vi.fn(),
  deleteCard: vi.fn(),
}));

import { getBoard, listBoards } from '../api/boards';
import { getCard, listCards } from '../api/cards';

const board = {
  id: 'b1',
  name: 'main',
  default_column: 'backlog',
  columns: [
    { name: 'backlog', color: '#000' },
    { name: 'next', color: '#000' },
  ],
  custom_fields: {},
} as BoardConfig;

function card(id: string, position: string, column = 'backlog'): Card {
  return { id, title: id, column, position } as Card;
}

function cardChange(cardId: string, boardName = 'main'): FileChange {
  return {
    type: 'modified',
    kind: 'card',
    board_name: boardName,
    card_id: cardId,
    path: `boards/${boardName}/cards/${cardId}.json`,
  };
}

function boardChange(boardName: string): FileChange {
  return {
    type: 'modified',
    kind: 'board',
    board_name: boardName,
    path: `boards/${boardName}/config.toml`,
  };
}

function wrapper({ children }: { children: ReactNode }) {
  return (
    <ToastProvider>
      <FileSyncProvider>{children}</FileSyncProvider>
    </ToastProvider>
  );
}

/** Mounts useBoard on a loaded three-card backlog. */
async function renderLoadedBoard(onBoardGone?: () => void) {
  vi.mocked(getBoard).mockResolvedValue(board);
  vi.mocked(listCards).mockResolvedValue([
    card('a', 'U'),
    card('b', 'UU'),
    card('c', 'UUU'),
  ]);

  const rendered = renderHook(() => useBoard('main', 0, onBoardGone), { wrapper });
  await waitFor(() => expect(rendered.result.current.cards).toHaveLength(3));
  return rendered;
}

function ids(cards: Card[]): string[] {
  return cards.map((c) => c.id);
}

beforeEach(() => {
  installFakeWebSocket();
});

afterEach(() => {
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe('useBoard file sync', () => {
  // The reported bug: a card reordered from the CLI kept its old rank on screen
  // until the page was reloaded.
  it('re-ranks a card the server reordered within its column', async () => {
    const { result } = await renderLoadedBoard();
    vi.mocked(getCard).mockResolvedValue(card('c', 'E'));

    act(() => FakeWebSocket.latest.emitChange(cardChange('c')));

    await waitFor(() => expect(ids(result.current.cards)).toEqual(['c', 'a', 'b']));
  });

  it('re-ranks a card the server moved to another column', async () => {
    const { result } = await renderLoadedBoard();
    vi.mocked(getCard).mockResolvedValue(card('a', 'UUUU', 'next'));

    act(() => FakeWebSocket.latest.emitChange(cardChange('a')));

    await waitFor(() => expect(ids(result.current.cards)).toEqual(['b', 'c', 'a']));
    expect(result.current.cards[2].column).toBe('next');
  });

  it('drops a card the server deleted', async () => {
    const { result } = await renderLoadedBoard();

    act(() => {
      FakeWebSocket.latest.emitChange({ ...cardChange('b'), type: 'deleted' });
    });

    await waitFor(() => expect(ids(result.current.cards)).toEqual(['a', 'c']));
  });

  it('ignores changes belonging to another board', async () => {
    const { result } = await renderLoadedBoard();

    act(() => FakeWebSocket.latest.emitChange(cardChange('z', 'other')));

    await waitFor(() => expect(getCard).not.toHaveBeenCalled());
    expect(ids(result.current.cards)).toEqual(['a', 'b', 'c']);
  });

  it('refreshes board and cards when the board config changes', async () => {
    const { result } = await renderLoadedBoard();
    const renamed = { ...board, columns: [{ name: 'todo', color: '#000' }] } as BoardConfig;
    vi.mocked(getBoard).mockResolvedValue(renamed);
    vi.mocked(listCards).mockResolvedValue([card('a', 'U', 'todo')]);

    act(() => FakeWebSocket.latest.emitChange(boardChange('main')));

    await waitFor(() => expect(result.current.board?.columns[0].name).toBe('todo'));
    expect(ids(result.current.cards)).toEqual(['a']);
  });

  it('reports the board as gone when refreshing it 404s', async () => {
    const onBoardGone = vi.fn();
    await renderLoadedBoard(onBoardGone);
    vi.mocked(getBoard).mockRejectedValue(new ApiError(404, 'board "main" not found'));
    vi.mocked(listCards).mockRejectedValue(new ApiError(404, 'board "main" not found'));

    act(() => FakeWebSocket.latest.emitChange(boardChange('main')));

    await waitFor(() => expect(onBoardGone).toHaveBeenCalledTimes(1));
  });

  it('treats a non-404 refresh failure as transient, not as a missing board', async () => {
    const onBoardGone = vi.fn();
    const { result } = await renderLoadedBoard(onBoardGone);
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    vi.mocked(getBoard).mockRejectedValue(new ApiError(500, 'boom'));
    vi.mocked(listCards).mockRejectedValue(new ApiError(500, 'boom'));

    act(() => FakeWebSocket.latest.emitChange(boardChange('main')));

    await waitFor(() => expect(console.warn).toHaveBeenCalled());
    expect(onBoardGone).not.toHaveBeenCalled();
    expect(ids(result.current.cards)).toEqual(['a', 'b', 'c']);
  });
});

describe('useBoards file sync', () => {
  it('refetches the list when any board appears or disappears', async () => {
    vi.mocked(listBoards).mockResolvedValue(['main']);
    const { result } = renderHook(() => useBoards(), { wrapper });
    await waitFor(() => expect(result.current.boards).toEqual(['main']));

    // Deliberately a different board than the one open - that is the case the old
    // board-filtered subscription dropped.
    vi.mocked(listBoards).mockResolvedValue(['main', 'second']);
    act(() => FakeWebSocket.latest.emitChange(boardChange('second')));

    await waitFor(() => expect(result.current.boards).toEqual(['main', 'second']));
  });

  it('ignores card changes', async () => {
    vi.mocked(listBoards).mockResolvedValue(['main']);
    const { result } = renderHook(() => useBoards(), { wrapper });
    await waitFor(() => expect(result.current.boards).toEqual(['main']));

    act(() => FakeWebSocket.latest.emitChange(cardChange('a')));

    await waitFor(() => expect(listBoards).toHaveBeenCalledTimes(1));
  });
});
