// @vitest-environment jsdom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { BoardCardsResponse, BoardConfig, Card } from '../api/types';
import { FileSyncProvider } from '../contexts/FileSyncContext';
import { ToastProvider } from '../contexts/ToastContext';
import { FakeWebSocket, installFakeWebSocket } from '../test/fakeWebSocket';
import { getBlockedCardIds } from '../utils/cardLinks';
import { useBoard } from './useBoards';

const board = (id: string): BoardConfig => ({ id, name: 'main', default_column: 'todo', columns: [{ name: 'todo', color: '', category: 'open' }] });
const source = (title: string, ref = 'remote:target'): Card => ({ id: 'source', title, alias: 'source', alias_explicit: false, column: 'todo', creator: 'tester', created_at_millis: 0, updated_at_millis: 0, blocked_by: [ref] });
const snapshot = (title: string): BoardCardsResponse => ({ cards: [source(title)], links: { resolved: { 'remote:target': { ref: 'remote:target', result: 'found', category: 'done' } }, blocks: {} } });
const response = (value: unknown) => new Response(JSON.stringify(value), { status: 200 });
const wrapper = ({ children }: { children: ReactNode }) => <ToastProvider><FileSyncProvider>{children}</FileSyncProvider></ToastProvider>;

beforeEach(() => { installFakeWebSocket(); });
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('board link metadata', () => {
  it('discards an older project response even when both projects use the same board name', async () => {
    let phase = 'old';
    let finishOld: ((value: Response) => void) | undefined;
    vi.stubGlobal('fetch', async (path: string) => {
      if (path === '/api/v1/boards/main') return response(board(phase));
      if (path === '/api/v1/boards/main/cards') {
        if (phase === 'old') return new Promise<Response>((resolve) => { finishOld = resolve; });
        return response(snapshot('New project'));
      }
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(({ generation }) => useBoard('main', generation), { wrapper, initialProps: { generation: 0 } });
    await waitFor(() => expect(finishOld).toBeDefined());
    phase = 'new';
    view.rerender({ generation: 1 });
    await waitFor(() => expect(view.result.current.cards[0]?.title).toBe('New project'));
    await act(async () => { finishOld?.(response({ ...snapshot('Old project'), links: { resolved: {}, blocks: {} } })); });
    expect(view.result.current.board?.id).toBe('new');
    expect(view.result.current.cards[0].title).toBe('New project');
    expect(view.result.current.links?.resolved['remote:target'].result).toBe('found');
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it('updates only local cards on a watcher event and keeps new foreign refs blocked until reload', async () => {
    const requests: string[] = [];
    vi.stubGlobal('fetch', async (path: string) => {
      requests.push(path);
      if (path === '/api/v1/boards/main') return response(board('local'));
      if (path === '/api/v1/boards/main/cards') return response(snapshot('Source'));
      if (path === '/api/v1/boards/main/cards/source') return response(source('Changed locally', 'remote:new'));
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(() => useBoard('main'), { wrapper });
    await waitFor(() => expect(view.result.current.cards).toHaveLength(1));
    act(() => FakeWebSocket.latest.emitChange({ kind: 'card', type: 'modified', board_name: 'main', card_id: 'source', path: 'boards/main/cards/source.json' }));
    await waitFor(() => expect(view.result.current.cards[0].title).toBe('Changed locally'));
    expect(view.result.current.links?.resolved['remote:target'].category).toBe('done');
    expect(requests).toEqual(['/api/v1/boards/main', '/api/v1/boards/main/cards', '/api/v1/boards/main/cards/source']);
    expect(getBlockedCardIds(view.result.current.cards, view.result.current.board!.columns, view.result.current.links, 'local').has('source')).toBe(true);
  });
});
