// @vitest-environment jsdom
import { act, cleanup, fireEvent, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { BoardCardsResponse, BoardConfig, Card } from '../api/types';
import { FileSyncProvider } from '../contexts/FileSyncContext';
import { ToastProvider, useToast } from '../contexts/ToastContext';
import { restoreCard } from '../api/cards';
import { FakeWebSocket, installFakeWebSocket } from '../test/fakeWebSocket';
import { getBlockedCardIds, incomingCardLinks, resolveCardLinks } from '../utils/cardLinks';
import { useBoard } from './useBoards';
import { useUndo } from './useUndo';

const board = (id: string): BoardConfig => ({ id, name: 'main', default_column: 'todo', columns: [{ name: 'todo', color: '', category: 'open' }, { name: 'done', color: '', category: 'done' }] });
const source = (title: string, ref = 'remote:target'): Card => ({ id: 'source', title, alias: 'source', alias_explicit: false, column: 'todo', creator: 'tester', created_at_millis: 0, updated_at_millis: 0, blocked_by: [ref] });
const snapshot = (title: string): BoardCardsResponse => ({ cards: [source(title)], links: { resolved: { 'remote:target': { ref: 'remote:target', result: 'found', category: 'done' } }, blocks: {} } });
const response = (value: unknown) => new Response(JSON.stringify(value), { status: 200 });
const wrapper = ({ children }: { children: ReactNode }) => <ToastProvider><FileSyncProvider>{children}</FileSyncProvider></ToastProvider>;
function notifyCard(id = 'source', type: 'created' | 'modified' | 'deleted' = 'modified') {
  act(() => FakeWebSocket.latest.emitChange({ kind: 'card', type, board_name: 'main', card_id: id, path: `boards/main/cards/${id}.json` }));
}

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

  it('resolves new foreign links after a local watcher event', async () => {
    const requests: string[] = [];
    let data = snapshot('Source');
    vi.stubGlobal('fetch', async (path: string) => {
      requests.push(path);
      if (path === '/api/v1/boards/main') return response(board('local'));
      if (path === '/api/v1/boards/main/cards') return response(data);
      if (path === '/api/v1/boards/main/cards/source') return response(source('Changed locally', 'remote:new'));
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(() => useBoard('main'), { wrapper });
    await waitFor(() => expect(view.result.current.cards).toHaveLength(1));
    const remote = { ...source('Remote target'), id: 'new', column: 'done' };
    data = { cards: [source('Changed locally', 'remote:new')], links: { resolved: {
      'remote:new': { ref: 'remote:new', result: 'found', category: 'done', board_id: 'remote', card: remote },
    }, blocks: {} } };
    act(() => FakeWebSocket.latest.emitChange({ kind: 'card', type: 'modified', board_name: 'main', card_id: 'source', path: 'boards/main/cards/source.json' }));
    await waitFor(() => expect(view.result.current.cards[0].title).toBe('Changed locally'));
    await waitFor(() => expect(view.result.current.links?.resolved['remote:new']?.result).toBe('found'));
    expect(view.result.current.links?.resolved['remote:target']).toBeUndefined();
    expect(resolveCardLinks(['remote:new'], view.result.current.cards, view.result.current.links, 'local')[0].card?.title).toBe('Remote target');
    expect(requests).toEqual(['/api/v1/boards/main', '/api/v1/boards/main/cards', '/api/v1/boards/main/cards/source', '/api/v1/boards/main/cards']);
    expect(getBlockedCardIds(view.result.current.cards, view.result.current.board!.columns, view.result.current.links, 'local').has('source')).toBe(false);
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it('updates qualified reverse links when a local link is added and removed', async () => {
    const target = { ...source('Local target'), id: 'target' };
    let changed = { ...source('Source'), blocked_by: [] as string[] };
    let data: BoardCardsResponse = { cards: [changed, target], links: { resolved: {}, blocks: {} } };
    vi.stubGlobal('fetch', async (path: string) => {
      if (path === '/api/v1/boards/main') return response(board('local'));
      if (path === '/api/v1/boards/main/cards') return response(data);
      if (path === '/api/v1/boards/main/cards/source') return response(changed);
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(() => useBoard('main'), { wrapper });
    await waitFor(() => expect(view.result.current.cards).toHaveLength(2));
    changed = { ...changed, blocked_by: ['local:target'] };
    data = { cards: [changed, target], links: { resolved: { 'local:target': {
      ref: 'local:target', result: 'found', board_id: 'local', card: target,
    } }, blocks: {} } };
    notifyCard();
    await waitFor(() => expect(incomingCardLinks('target', view.result.current.cards, view.result.current.links, 'local').map((link) => link.card?.title)).toEqual(['Source']));
    changed = { ...changed, blocked_by: [] };
    data = { cards: [changed, target], links: { resolved: {}, blocks: {} } };
    notifyCard();
    await waitFor(() => expect(view.result.current.cards[0].blocked_by).toEqual([]));
    await waitFor(() => expect(view.result.current.links?.resolved).toEqual({}));
    expect(incomingCardLinks('target', view.result.current.cards, view.result.current.links, 'local')).toEqual([]);
  });

  it('resolves a related link added without changing blockers', async () => {
    let changed = source('Source');
    let data = snapshot('Source');
    vi.stubGlobal('fetch', async (path: string) => {
      if (path === '/api/v1/boards/main') return response(board('local'));
      if (path === '/api/v1/boards/main/cards') return response(data);
      if (path === '/api/v1/boards/main/cards/source') return response(changed);
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(() => useBoard('main'), { wrapper });
    await waitFor(() => expect(view.result.current.cards).toHaveLength(1));
    changed = { ...changed, related: ['remote:peer'] };
    data = { ...data, links: { resolved: { ...data.links!.resolved,
      'remote:peer': { ref: 'remote:peer', result: 'found', card: source('Related remote card') },
    }, blocks: {} } };
    notifyCard();
    await waitFor(() => expect(resolveCardLinks(['remote:peer'], view.result.current.cards, view.result.current.links, 'local')[0].card?.title).toBe('Related remote card'));
  });

  it.each(['created', 'modified', 'deleted'] as const)('refreshes metadata when a referenced local target is %s', async (type) => {
    const waiting = source('Waiting card', 'local:target');
    const target = { ...source('Local target'), id: 'target', blocked_by: [] };
    let data: BoardCardsResponse = { cards: type === 'created' ? [waiting] : [waiting, target],
      links: { resolved: { 'local:target': { ref: 'local:target', result: type === 'created' ? 'absent' : 'found', category: 'open' } }, blocks: {} } };
    vi.stubGlobal('fetch', async (path: string) => {
      if (path === '/api/v1/boards/main') return response(board('local'));
      if (path === '/api/v1/boards/main/cards') return response(data);
      if (path === '/api/v1/boards/main/cards/target') return response({ ...target, column: 'done' });
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(() => useBoard('main'), { wrapper });
    await waitFor(() => expect(view.result.current.cards.length).toBe(data.cards.length));
    data = { cards: [], links: { resolved: { 'local:target': {
      ref: 'local:target', result: type === 'deleted' ? 'absent' : 'found', category: 'done',
    } }, blocks: {} } };
    notifyCard('target', type);
    await waitFor(() => expect(view.result.current.links?.resolved['local:target']?.category).toBe('done'));
    expect(view.result.current.cards.some((card) => card.id === 'target')).toBe(type !== 'deleted');
    expect(getBlockedCardIds(view.result.current.cards, view.result.current.board!.columns, view.result.current.links, 'local').has('source')).toBe(type === 'deleted');
  });

  it('keeps ordinary edits lightweight and foreign column changes pending until reload', async () => {
    let reads = 0;
    let data = snapshot('Source');
    data.links!.resolved['remote:target'].category = 'open';
    vi.stubGlobal('fetch', async (path: string) => {
      if (path === '/api/v1/boards/main') return response(board('local'));
      if (path === '/api/v1/boards/main/cards') { reads++; return response(data); }
      if (path === '/api/v1/boards/main/cards/source') return response(source('Title changed'));
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(() => useBoard('main'), { wrapper });
    await waitFor(() => expect(view.result.current.cards).toHaveLength(1));
    data = snapshot('Title changed');
    notifyCard();
    await waitFor(() => expect(view.result.current.cards[0].title).toBe('Title changed'));
    expect(reads).toBe(1);
    expect(getBlockedCardIds(view.result.current.cards, view.result.current.board!.columns, view.result.current.links, 'local').has('source')).toBe(true);
    await act(async () => { await view.result.current.refresh(); });
    expect(reads).toBe(2);
    expect(getBlockedCardIds(view.result.current.cards, view.result.current.board!.columns, view.result.current.links, 'local').has('source')).toBe(false);
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it.each(['linked target', 'card with foreign links'])('refreshes metadata when Undo restores a %s', async (scenario) => {
    const target = scenario === 'linked target';
    const restored = target ? { ...source('Restored target'), id: 'target', column: 'done', blocked_by: [] } : source('Restored source');
    const waiting = { ...source('Waiting card', target ? 'local:target' : restored.id), id: 'waiting' };
    let present = true;
    let reads = 0;
    vi.stubGlobal('fetch', async (path: string, init?: RequestInit) => {
      if (path === '/api/v1/boards/main') return response(board('local'));
      if (path === '/api/v1/boards/main/cards') {
        reads++;
        const ref = target ? 'local:target' : 'remote:target';
        return response({ cards: present ? [waiting, restored] : [waiting], links: {
          resolved: present ? { [ref]: { ref, result: 'found', category: 'done' } } : {}, blocks: {},
        } });
      }
      if (path === `/api/v1/boards/main/cards/${restored.id}` && init?.method === 'DELETE') {
        present = false;
        return response({});
      }
      if (path === '/api/v1/boards/main/cards/restore' && init?.method === 'POST') {
        present = true;
        return response(restored);
      }
      throw new Error(`Unexpected request ${init?.method ?? 'GET'} ${path}`);
    });
    const view = renderHook(() => {
      const state = useBoard('main');
      const { showToast } = useToast();
      const undo = useUndo({ boardName: 'main', cards: state.cards, board: state.board,
        moveCard: state.moveCard, updateCard: state.updateCard, deleteCard: state.deleteCard,
        restoreCard, addCardToState: state.addCardToState, showToast });
      return { state, undo };
    }, { wrapper });
    await waitFor(() => expect(view.result.current.state.cards).toHaveLength(2));
    await act(async () => { await view.result.current.state.deleteCard(restored.id); });
    act(() => view.result.current.undo.pushUndo({ type: 'delete', card: restored, column: restored.column, position: 0 }));
    notifyCard(restored.id, 'deleted');
    await waitFor(() => expect(view.result.current.state.links?.resolved).toEqual({}));
    fireEvent.keyDown(document, { key: 'z', metaKey: true });
    await waitFor(() => expect(view.result.current.state.cards).toHaveLength(2));
    notifyCard(restored.id, 'created');
    await waitFor(() => expect(view.result.current.state.links?.resolved[target ? 'local:target' : 'remote:target']?.result).toBe('found'));
    expect(reads).toBe(3);
    const state = view.result.current.state;
    const blocked = getBlockedCardIds(state.cards, state.board!.columns, state.links, 'local');
    expect(blocked.has(target ? waiting.id : restored.id)).toBe(false);
    expect(state.links?.resolved[target ? 'local:target' : 'remote:target']?.result).toBe('found');
  });

  it('keeps the newest metadata response without replacing current cards', async () => {
    let reads = 0;
    let changed = source('First change', 'remote:first');
    let finishOld: ((value: Response) => void) | undefined;
    vi.stubGlobal('fetch', async (path: string) => {
      if (path === '/api/v1/boards/main') return response(board('local'));
      if (path === '/api/v1/boards/main/cards') {
        reads++;
        if (reads === 1) return response(snapshot('Source'));
        if (reads === 2) return new Promise<Response>((resolve) => { finishOld = resolve; });
        return response({ cards: [source('Stale list copy')], links: { resolved: {
          'remote:second': { ref: 'remote:second', result: 'found' },
        }, blocks: {} } });
      }
      if (path === '/api/v1/boards/main/cards/source') return response(changed);
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(() => useBoard('main'), { wrapper });
    await waitFor(() => expect(view.result.current.cards).toHaveLength(1));
    notifyCard();
    await waitFor(() => expect(finishOld).toBeDefined());
    changed = source('Second change', 'remote:second');
    notifyCard();
    await waitFor(() => expect(view.result.current.links?.resolved['remote:second']?.result).toBe('found'));
    await act(async () => { finishOld?.(response(snapshot('Old metadata'))); });
    expect(view.result.current.cards[0].title).toBe('Second change');
    expect(view.result.current.links?.resolved['remote:second']?.result).toBe('found');
    expect(view.result.current.links?.resolved['remote:target']).toBeUndefined();
  });

  it('discards a local-event metadata response after a project switch', async () => {
    let phase = 'old';
    let reads = 0;
    let finishOld: ((value: Response) => void) | undefined;
    vi.stubGlobal('fetch', async (path: string) => {
      if (path === '/api/v1/boards/main') return response(board(phase));
      if (path === '/api/v1/boards/main/cards') {
        if (phase === 'new') return response(snapshot('New project'));
        if (++reads === 1) return response(snapshot('Old project'));
        return new Promise<Response>((resolve) => { finishOld = resolve; });
      }
      if (path === '/api/v1/boards/main/cards/source') return response(source('Old project changed', 'remote:new'));
      throw new Error(`Unexpected request ${path}`);
    });
    const view = renderHook(({ generation }) => useBoard('main', generation), { wrapper, initialProps: { generation: 0 } });
    await waitFor(() => expect(view.result.current.cards).toHaveLength(1));
    notifyCard();
    await waitFor(() => expect(finishOld).toBeDefined());
    phase = 'new';
    view.rerender({ generation: 1 });
    await waitFor(() => expect(view.result.current.cards[0]?.title).toBe('New project'));
    await act(async () => { finishOld?.(response({ cards: [], links: { resolved: {}, blocks: {} } })); });
    expect(view.result.current.board?.id).toBe('new');
    expect(view.result.current.links?.resolved['remote:target']?.result).toBe('found');
  });
});
