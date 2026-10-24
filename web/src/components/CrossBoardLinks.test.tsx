// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from '../App';
import type { BoardConfig, BoardLinks, Card, LinkedCardResponse } from '../api/types';
import { ThemeProvider } from '../contexts/ThemeContext';
import { CompactModeProvider } from '../contexts/CompactModeContext';
import { SlimModeProvider } from '../contexts/SlimModeContext';
import { ToastProvider } from '../contexts/ToastContext';
import { FakeWebSocket, installFakeWebSocket } from '../test/fakeWebSocket';

const board: BoardConfig = { id: 'local', name: 'main', default_column: 'todo', columns: [{ name: 'todo', color: '', category: 'open' }], custom_fields: { priority: { type: 'string' } } };
const card = (id: string, title: string, updates: Partial<Card> = {}): Card => ({
  id, title, alias: id, alias_explicit: false, column: 'todo', creator: 'tester', created_at_millis: 0, updated_at_millis: 0, ...updates,
});
const source = card('source', 'Local source', { blocked_by: ['remote:shared'] });
const foreign = card('shared', 'Foreign card', { column: 'done', description: 'Remote **description** ABC-42', comments: [{ id: 'comment', author: 'remote author', body: 'Remote comment', created_at_millis: 0 }] });
const remoteBoard: BoardConfig = { ...board, id: 'remote', link_rules: [{ name: 'Tracker', pattern: '(ABC-\\d+)', url: 'https://remote.example.com/{1}' }] };
const foreignLink = { ref: 'remote:shared', result: 'found' as const, board_id: 'remote', board: 'main', project_path: '/remote', project_name: 'Remote project', category: 'done', card: foreign };
const response = (value: unknown) => new Response(JSON.stringify(value), { status: 200 });

function setup(initial = '/board/main?card=source&sort=priority') {
  window.history.replaceState({}, '', initial);
  let links: BoardLinks = { resolved: { 'remote:shared': foreignLink }, blocks: {} };
  const views = new Map<string, LinkedCardResponse>([
    ['remote:shared', { ...foreignLink, read_only: true, board_config: remoteBoard,
      links: { resolved: {}, blocks: { shared: [{ ref: 'local:source', result: 'found', card: source, board_id: 'local' }] } } }],
    ...[source, card('shared', 'Local duplicate')].map((local): [string, LinkedCardResponse] => [`local:${local.id}`, {
      ref: `local:${local.id}`, result: 'found', read_only: true, board_id: 'local', board: 'main',
      project_path: '/local', project_name: 'Local project', card: local, board_config: board,
    }]),
  ]);
  let resolveView: (() => Promise<Response>) | undefined;
  const requests: { path: string; method: string }[] = [];
  vi.stubGlobal('fetch', async (path: string, init?: RequestInit) => {
    requests.push({ path, method: init?.method ?? 'GET' });
    if (path === '/api/v1/boards') return response({ boards: ['main'] });
    if (path === '/api/v1/project') return response({ name: 'Local project', project_path: '/local', favicon: {} });
    if (path === '/api/v1/boards/main') return response(board);
    if (path === '/api/v1/boards/main/cards') return response({ cards: [source, card('shared', 'Local duplicate')], links });
    if (path === '/api/v1/all-boards') return response({ boards: [] });
    if (path.startsWith('/api/v1/linked-cards/')) {
      if (resolveView && path.endsWith('remote%3Ashared')) return resolveView();
      const view = views.get(decodeURIComponent(path.slice('/api/v1/linked-cards/'.length)));
      if (view) return response(view);
    }
    throw new Error(`Unexpected request: ${init?.method ?? 'GET'} ${path}`);
  });
  return {
    requests,
    setLinks: (next: BoardLinks) => { links = next; },
    setView: (ref: string, next: LinkedCardResponse) => { views.set(ref, next); },
    delayView: (read: () => Promise<Response>) => { resolveView = read; },
    render: () => render(<ThemeProvider><CompactModeProvider><SlimModeProvider><ToastProvider><App /></ToastProvider></SlimModeProvider></CompactModeProvider></ThemeProvider>),
  };
}

beforeEach(() => {
  vi.stubGlobal('matchMedia', (media: string) => Object.assign(new EventTarget(), { matches: false, media }));
  installFakeWebSocket();
});
afterEach(() => { cleanup(); localStorage.clear(); vi.unstubAllGlobals(); });

describe('cross-board links through the app', () => {
  it.each(['ambiguous_board', 'unreadable'] as const)('keeps a qualified current-board URL unresolved when lookup is %s', async (result) => {
    const fixture = setup('/board/main?card=local%3Ashared&sort=priority');
    fixture.setView('local:shared', { ref: 'local:shared', result, read_only: true, reason: 'Board identity is uncertain' });
    fixture.render();
    const unavailable = await screen.findByRole('region', { name: 'Unavailable card' });
    expect(within(unavailable).getByText('Board identity is uncertain')).toBeTruthy();
    expect(screen.queryByPlaceholderText('Card title')).toBeNull();
    expect(new URLSearchParams(window.location.search).get('card')).toBe('local:shared');
    expect(fixture.requests.some((request) => request.path === '/api/v1/linked-cards/local%3Ashared')).toBe(true);
  });

  it('opens a qualified current-board card editable only after the server confirms its identity', async () => {
    const fixture = setup('/board/main?card=local%3Ashared&sort=priority');
    fixture.render();
    await waitFor(() => expect((screen.getByPlaceholderText('Card title') as HTMLTextAreaElement).value).toBe('Local duplicate'));
    expect(fixture.requests.some((request) => request.path === '/api/v1/linked-cards/local%3Ashared')).toBe(true);
    expect(new URLSearchParams(window.location.search).get('card')).toBe('shared');
    expect(new URLSearchParams(window.location.search).get('sort')).toBe('priority');
  });

  it('keeps a comment draft on the source before opening another project', async () => {
    const fixture = setup();
    fixture.render();
    const draft = await screen.findByPlaceholderText('Add a comment...');
    fireEvent.change(draft, { target: { value: 'Unposted comment' } });
    fireEvent.click(within(screen.getByRole('region', { name: 'Blocked by' })).getByRole('button', { name: /Foreign card/ }));
    expect(new URLSearchParams(window.location.search).get('card')).toBe('source');
    expect((draft as HTMLTextAreaElement).value).toBe('Unposted comment');
    expect(fixture.requests.some((request) => request.path.startsWith('/api/v1/linked-cards'))).toBe(false);
  });
  it('opens another project read-only without switching boards and returns to a local card', async () => {
    const fixture = setup();
    fixture.render();
    const blockers = await screen.findByRole('region', { name: 'Blocked by' });
    fireEvent.click(within(blockers).getByRole('button', { name: /Foreign card/ }));
    const dialog = await screen.findByRole('dialog', { name: 'Foreign card' });
    expect(within(dialog).getByText(/Read only.*Remote project/)).toBeTruthy();
    expect(within(dialog).getByText('Remote comment')).toBeTruthy();
    expect(within(dialog).getByRole('link', { name: 'ABC-42' }).getAttribute('href')).toBe('https://remote.example.com/ABC-42');
    expect(within(dialog).queryByRole('textbox')).toBeNull();
    expect(within(dialog).queryByRole('combobox')).toBeNull();
    expect(within(dialog).queryByRole('button', { name: /delete|save|post/i })).toBeNull();
    expect(window.location.pathname).toBe('/board/main');
    expect(new URLSearchParams(window.location.search).get('card')).toBe('remote:shared');
    expect(new URLSearchParams(window.location.search).get('sort')).toBe('priority');
    fireEvent.click(within(dialog).getByRole('button', { name: /Local source/ }));
    await waitFor(() => expect((screen.getByPlaceholderText('Card title') as HTMLTextAreaElement).value).toBe('Local source'));
    expect(fixture.requests.every((request) => request.method === 'GET')).toBe(true);
    expect(fixture.requests.some((request) => request.path.includes('/switch'))).toBe(false);
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it('keeps a foreign URL open while loading, then ignores a response after close', async () => {
    const fixture = setup('/board/main?card=remote%3Ashared&sort=priority');
    let finish: ((value: Response) => void) | undefined;
    fixture.delayView(() => new Promise<Response>((resolve) => { finish = resolve; }));
    fixture.render();
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByRole('status').textContent).toBe('Loading card...');
    expect(new URLSearchParams(window.location.search).get('card')).toBe('remote:shared');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await act(async () => { finish?.(response({ ...foreignLink, read_only: true, board_config: remoteBoard })); });
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(new URLSearchParams(window.location.search).get('card')).toBeNull();
  });

  it('refreshes foreign blocker state only when the board reloads', async () => {
    const fixture = setup('/board/main');
    fixture.render();
    await screen.findByText('Local source');
    expect(screen.queryByText('Blocked')).toBeNull();
    fixture.setLinks({ resolved: { 'remote:shared': { ...foreignLink, category: 'open' } }, blocks: {} });
    expect(screen.queryByText('Blocked')).toBeNull();
    fireEvent.click(screen.getByTitle('Refresh'));
    await screen.findByText('Blocked');
    expect(fixture.requests.filter((request) => request.path === '/api/v1/boards/main/cards')).toHaveLength(2);
    expect(fixture.requests.some((request) => request.path.startsWith('/api/v1/linked-cards'))).toBe(false);
    expect(FakeWebSocket.instances).toHaveLength(1);
  });
});
