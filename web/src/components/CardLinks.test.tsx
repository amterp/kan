// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { useState } from 'react';
import type { ReactNode } from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { BoardConfig, Card, UpdateCardInput } from '../api/types';
import { updateCard } from '../api/cards';
import { CompactModeProvider } from '../contexts/CompactModeContext';
import { SlimModeProvider } from '../contexts/SlimModeContext';
import { ToastProvider } from '../contexts/ToastContext';
import { ThemeProvider } from '../contexts/ThemeContext';
import { useUrlState } from '../hooks/useUrlState';
import Board from './Board';
import CardEditModal from './CardEditModal';

const board: BoardConfig = {
  id: 'board-main', name: 'main', default_column: 'todo',
  columns: [{ name: 'todo', color: '#111', category: 'open' }, { name: 'done', color: '#222', category: 'done' }],
};
const makeCard = (id: string, updates: Partial<Card> = {}): Card => ({
  id, alias: id, alias_explicit: false, title: id, column: 'todo', creator: 'tester',
  created_at_millis: 0, updated_at_millis: 0, ...updates,
});

function Providers({ children }: { children: ReactNode }) {
  return <ThemeProvider><CompactModeProvider><SlimModeProvider><ToastProvider>{children}</ToastProvider></SlimModeProvider></CompactModeProvider></ThemeProvider>;
}

beforeEach(() => {
  vi.stubGlobal('matchMedia', (media: string) => Object.assign(new EventTarget(), { matches: false, media }));
});

afterEach(() => {
  cleanup();
  localStorage.clear();
  vi.unstubAllGlobals();
});

describe('blocked marker', () => {
  it.each(['normal', 'compact', 'slim'])('updates from a hidden blocker in %s mode', (mode) => {
    localStorage.setItem('kan-compact-mode', String(mode === 'compact'));
    localStorage.setItem('kan-slim-mode', String(mode === 'slim'));
    const source = makeCard('source', { title: 'Waiting work', blocked_by: ['target'] });
    const target = makeCard('target', { title: 'Hidden dependency', column: 'done' });
    const renderBoard = (cards: Card[]) => (
      <MemoryRouter><Providers>
        <Board board={board} cards={cards} filterQuery="Waiting work" onMoveCard={async () => {}}
          onCreateCard={async () => undefined} onUpdateCard={async () => {}} onDeleteCard={async () => {}}
          onOpenCard={() => {}} />
      </Providers></MemoryRouter>
    );
    const view = render(renderBoard([source, target]));
    expect(screen.queryByText('Hidden dependency')).toBeNull();
    expect(screen.queryByText('Blocked')).toBeNull();

    view.rerender(renderBoard([source, { ...target, column: 'todo' }]));
    expect(screen.getByText('Blocked').getAttribute('title')).toBe('Waiting for blockers to finish');

    view.rerender(renderBoard([source, target]));
    expect(screen.queryByText('Blocked')).toBeNull();
  });
});

function ModalHarness({ initialCards }: { initialCards: Card[] }) {
  const [cards, setCards] = useState(initialCards);
  const { cardId, openCard, closeCard } = useUrlState();
  const location = useLocation();
  const card = cards.find((candidate) => candidate.id === cardId);
  const save = async (updates: UpdateCardInput) => {
    if (!card) return;
    const updated = await updateCard(board.name, card.id, updates);
    setCards((current) => current.map((candidate) => candidate.id === updated.id ? updated : candidate));
  };
  return <>
    <output aria-label="Location">{location.pathname}{location.search}</output>
    {card && <CardEditModal key={card.id} card={card} cards={cards} board={board} onOpenCard={openCard}
      onSave={save} onDelete={async () => {}} onClose={closeCard} />}
  </>;
}

function renderModal(cards: Card[]) {
  return render(
    <MemoryRouter initialEntries={['/board/main?card=source&sort=priority&slim=true']}>
      <Providers><Routes><Route path="/board/:boardName" element={<ModalHarness initialCards={cards} />} /></Routes></Providers>
    </MemoryRouter>
  );
}

describe('modal blockers', () => {
  it('lists known and unavailable blockers and incoming blockers, with no fetch', () => {
    const fetch = vi.fn(() => { throw new Error('unexpected fetch'); });
    vi.stubGlobal('fetch', fetch);
    renderModal([
      makeCard('source', { blocked_by: ['target', 'missing', 'remote:card'] }),
      makeCard('target', { title: 'Dependency', column: 'done' }),
      makeCard('waiter', { title: 'Waiting work', blocked_by: ['source'] }),
      makeCard('related', { title: 'Related only', related: ['source'] }),
    ]);
    const blockers = within(screen.getByRole('region', { name: 'Blocked by' }));
    expect(blockers.getByRole('button', { name: /Dependency/ }).textContent).toContain('done');
    expect(blockers.getByText('missing')).toBeTruthy();
    expect(blockers.getByText('remote:card')).toBeTruthy();
    expect(blockers.getAllByText('Unavailable blocker')).toHaveLength(2);
    const blocks = within(screen.getByRole('region', { name: 'Blocks' }));
    expect(blocks.getByRole('button', { name: /Waiting work/ })).toBeTruthy();
    expect(blocks.queryByText('Related only')).toBeNull();
    expect(fetch).not.toHaveBeenCalled();
  });

  it('saves edits before opening a blocker, resets modal state, and keeps URL parameters', async () => {
    const source = makeCard('source', { blocked_by: ['target'], comments: [{ id: 'comment-source', body: 'Source comment', author: 'tester', created_at_millis: 0 }] });
    const target = makeCard('target', { title: 'Dependency', comments: [{ id: 'comment-target', body: 'Target comment', author: 'tester', created_at_millis: 0 }] });
    let completeSave: ((response: Response) => void) | undefined;
    const requests: { path: string; updates: UpdateCardInput }[] = [];
    vi.stubGlobal('fetch', (path: string, init: RequestInit) => {
      requests.push({ path, updates: JSON.parse(String(init.body)) });
      return new Promise<Response>((resolve) => { completeSave = resolve; });
    });
    renderModal([source, target]);
    fireEvent.change(screen.getByPlaceholderText('Card title'), { target: { value: 'Edited source' } });
    fireEvent.click(within(screen.getByRole('region', { name: 'Blocked by' })).getByRole('button', { name: /Dependency/ }));
    expect(requests).toEqual([{ path: '/api/v1/boards/main/cards/source', updates: { title: 'Edited source', description: '', column: 'todo' } }]);
    expect(screen.getByLabelText('Location').textContent).toContain('card=source');
    expect(screen.getByText('Source comment')).toBeTruthy();

    await act(async () => { completeSave?.(new Response(JSON.stringify({ ...source, title: 'Edited source' }), { status: 200 })); });
    await waitFor(() => expect(screen.getByLabelText('Location').textContent).toBe('/board/main?card=target&sort=priority&slim=true'));
    expect((screen.getByPlaceholderText('Card title') as HTMLTextAreaElement).value).toBe(target.title);
    expect(screen.getByText('Target comment')).toBeTruthy();
    expect(screen.queryByText('Source comment')).toBeNull();
    expect(requests).toHaveLength(1);
  });

  it('keeps edits and the source card open if saving fails', async () => {
    vi.stubGlobal('fetch', async () => new Response(JSON.stringify({ error: 'Disk full' }), { status: 500 }));
    renderModal([makeCard('source', { blocked_by: ['target'] }), makeCard('target', { title: 'Dependency' })]);
    fireEvent.change(screen.getByPlaceholderText('Card title'), { target: { value: 'Unsaved title' } });
    fireEvent.click(within(screen.getByRole('region', { name: 'Blocked by' })).getByRole('button', { name: /Dependency/ }));
    await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('Disk full'));
    expect((screen.getByPlaceholderText('Card title') as HTMLTextAreaElement).value).toBe('Unsaved title');
    expect(screen.getByLabelText('Location').textContent).toContain('card=source');
  });

  it('keeps an invalid blank title open rather than discarding edits', async () => {
    renderModal([makeCard('source', { blocked_by: ['target'] }), makeCard('target', { title: 'Dependency' })]);
    fireEvent.change(screen.getByPlaceholderText('Card title'), { target: { value: '' } });
    fireEvent.click(within(screen.getByRole('region', { name: 'Blocked by' })).getByRole('button', { name: /Dependency/ }));
    expect(screen.getByRole('alert').textContent).toBe('Enter a title before opening another card.');
    expect(screen.getByLabelText('Location').textContent).toContain('card=source');
  });

  it('opens an incoming blocker without saving an unchanged card', async () => {
    const fetch = vi.fn(() => { throw new Error('unexpected fetch'); });
    vi.stubGlobal('fetch', fetch);
    renderModal([makeCard('source'), makeCard('waiter', { title: 'Waiting work', blocked_by: ['source'] })]);
    fireEvent.click(within(screen.getByRole('region', { name: 'Blocks' })).getByRole('button', { name: /Waiting work/ }));
    await waitFor(() => expect(screen.getByLabelText('Location').textContent).toContain('card=waiter'));
    expect(fetch).not.toHaveBeenCalled();
  });
});
