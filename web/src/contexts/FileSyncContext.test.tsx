// @vitest-environment jsdom
import { act, render, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { FileChange } from '../api/types';
import { FakeWebSocket, installFakeWebSocket } from '../test/fakeWebSocket';
import {
  FileSyncProvider,
  useFileSyncStatus,
  useFileSyncSubscription,
} from './FileSyncContext';

const RECONNECT_DELAY = 2000;
const MAX_RECONNECT_ATTEMPTS = 10;

const cardChange: FileChange = {
  type: 'modified',
  kind: 'card',
  board_name: 'main',
  card_id: 'a1',
  path: 'boards/main/cards/a1.json',
};

function wrapper({ children }: { children: ReactNode }) {
  return <FileSyncProvider>{children}</FileSyncProvider>;
}

beforeEach(() => {
  installFakeWebSocket();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('FileSyncProvider', () => {
  it('opens exactly one socket however many consumers subscribe', () => {
    function Consumer() {
      useFileSyncSubscription(() => {});
      return null;
    }

    render(
      <FileSyncProvider>
        <Consumer />
        <Consumer />
        <Consumer />
      </FileSyncProvider>
    );

    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(FakeWebSocket.latest.url).toMatch(/\/api\/v1\/ws$/);
  });

  it('delivers each change to every subscriber', () => {
    const seen: string[] = [];
    const { result } = renderHook(
      () => {
        useFileSyncSubscription((c) => seen.push(`first:${c.card_id}`));
        useFileSyncSubscription((c) => seen.push(`second:${c.card_id}`));
      },
      { wrapper }
    );

    act(() => FakeWebSocket.latest.emitChange(cardChange));

    expect(result.current).toBeUndefined();
    expect(seen).toEqual(['first:a1', 'second:a1']);
  });

  it('stops delivering to a subscriber that unmounted', () => {
    const handler = vi.fn();
    const { unmount } = renderHook(() => useFileSyncSubscription(handler), { wrapper });
    const socket = FakeWebSocket.latest;

    act(() => socket.emitChange(cardChange));
    expect(handler).toHaveBeenCalledTimes(1);

    unmount();
    act(() => socket.emitChange(cardChange));
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it('calls the latest handler without resubscribing', () => {
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = renderHook(
      ({ handler }: { handler: () => void }) => useFileSyncSubscription(handler),
      { wrapper, initialProps: { handler: first } }
    );

    rerender({ handler: second });
    act(() => FakeWebSocket.latest.emitChange(cardChange));

    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledWith(cardChange);
  });

  it('ignores the connection greeting', () => {
    const handler = vi.fn();
    renderHook(() => useFileSyncSubscription(handler), { wrapper });

    act(() => FakeWebSocket.latest.emitGreeting());

    expect(handler).not.toHaveBeenCalled();
  });

  it('survives a malformed frame and keeps serving later ones', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
    const handler = vi.fn();
    renderHook(() => useFileSyncSubscription(handler), { wrapper });
    const socket = FakeWebSocket.latest;

    act(() => socket.deliver('not json'));
    expect(handler).not.toHaveBeenCalled();

    act(() => socket.emitChange(cardChange));
    expect(handler).toHaveBeenCalledWith(cardChange);
  });

  it('tracks connection status', async () => {
    const { result } = renderHook(() => useFileSyncStatus(), { wrapper });
    expect(result.current.connected).toBe(false);

    act(() => FakeWebSocket.latest.open());
    await waitFor(() => expect(result.current.connected).toBe(true));
    expect(result.current.reconnecting).toBe(false);
    expect(result.current.failed).toBe(false);
  });

  it('reconnects after the server drops the connection', async () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useFileSyncStatus(), { wrapper });

    act(() => FakeWebSocket.latest.open());
    act(() => FakeWebSocket.latest.serverClose());
    expect(result.current.reconnecting).toBe(true);

    act(() => vi.advanceTimersByTime(RECONNECT_DELAY));
    expect(FakeWebSocket.instances).toHaveLength(2);

    act(() => FakeWebSocket.latest.open());
    expect(result.current.connected).toBe(true);
    expect(result.current.reconnecting).toBe(false);
  });

  it('gives up after the attempt limit and reports failure', () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useFileSyncStatus(), { wrapper });

    for (let i = 0; i < MAX_RECONNECT_ATTEMPTS; i++) {
      act(() => FakeWebSocket.latest.serverClose());
      act(() => vi.advanceTimersByTime(RECONNECT_DELAY));
    }
    expect(FakeWebSocket.instances).toHaveLength(MAX_RECONNECT_ATTEMPTS + 1);

    act(() => FakeWebSocket.latest.serverClose());
    expect(result.current.failed).toBe(true);
    expect(result.current.reconnecting).toBe(false);

    act(() => vi.advanceTimersByTime(RECONNECT_DELAY * 5));
    expect(FakeWebSocket.instances).toHaveLength(MAX_RECONNECT_ATTEMPTS + 1);
  });

  it('does not reconnect when the provider unmounts', () => {
    vi.useFakeTimers();
    const { unmount } = renderHook(() => useFileSyncStatus(), { wrapper });

    act(() => FakeWebSocket.latest.open());
    unmount();

    act(() => vi.advanceTimersByTime(RECONNECT_DELAY * 3));
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it('is inert outside a provider, for the docs-only build', () => {
    const { result } = renderHook(() => {
      useFileSyncSubscription(() => {});
      return useFileSyncStatus();
    });

    expect(FakeWebSocket.instances).toHaveLength(0);
    expect(result.current).toEqual({ connected: false, reconnecting: false, failed: false });
  });
});
