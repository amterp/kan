/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useCallback, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { FileChange } from '../api/types';

interface WebSocketMessage {
  type: string;
  data: FileChange | { message: string };
}

export type FileChangeHandler = (change: FileChange) => void;

export interface FileSyncStatus {
  connected: boolean;
  reconnecting: boolean;
  failed: boolean; // True when max reconnect attempts reached
}

interface FileSyncContextValue extends FileSyncStatus {
  subscribe: (handler: FileChangeHandler) => () => void;
}

const FileSyncContext = createContext<FileSyncContextValue | null>(null);

const RECONNECT_DELAY = 2000; // 2 seconds
const MAX_RECONNECT_ATTEMPTS = 10;

/**
 * FileSyncProvider owns the single WebSocket carrying file change notifications
 * from the server, and fans each change out to every subscriber.
 *
 * One connection, shared: the board, the board list, and the project config all
 * need these events, and a hook-per-consumer would open a socket per consumer.
 * Deliberately unfiltered, too - filtering by board here would hide exactly the
 * events the board list needs, since those are about other boards by definition.
 * Subscribers decide what concerns them.
 *
 * Mount this only where a backend exists; the docs-only build has none.
 */
export function FileSyncProvider({ children }: { children: ReactNode }) {
  const [connected, setConnected] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [failed, setFailed] = useState(false);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Lets the reconnect timer call the latest connect() without connect having
  // to reference its own binding before it's declared.
  const connectRef = useRef<(() => void) | null>(null);

  const handlersRef = useRef<Set<FileChangeHandler>>(new Set());

  const subscribe = useCallback((handler: FileChangeHandler) => {
    handlersRef.current.add(handler);
    return () => {
      handlersRef.current.delete(handler);
    };
  }, []);

  const handleMessage = useCallback((event: MessageEvent) => {
    let message: WebSocketMessage;
    try {
      message = JSON.parse(event.data);
    } catch (err) {
      console.error(
        'Failed to parse WebSocket message; this change will not reach the UI until a reload. ' +
          'Check that the server and frontend are the same version:',
        err
      );
      return;
    }

    if (message.type !== 'file_change') {
      return; // Currently only the 'connected' greeting
    }

    const change = message.data as FileChange;
    // Copy first: a handler may unsubscribe (unmount) during the fan-out.
    for (const handler of [...handlersRef.current]) {
      handler(change);
    }
  }, []);

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws`;

    try {
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        setConnected(true);
        setReconnecting(false);
        setFailed(false);
        reconnectAttemptsRef.current = 0;
      };

      ws.onclose = () => {
        setConnected(false);
        wsRef.current = null;

        if (reconnectAttemptsRef.current < MAX_RECONNECT_ATTEMPTS) {
          setReconnecting(true);
          reconnectAttemptsRef.current++;
          reconnectTimeoutRef.current = setTimeout(() => connectRef.current?.(), RECONNECT_DELAY);
        } else {
          setReconnecting(false);
          setFailed(true);
        }
      };

      ws.onerror = () => {
        // Error will be followed by close event
        console.warn('WebSocket error occurred');
      };

      ws.onmessage = handleMessage;
    } catch (err) {
      console.error('Failed to create WebSocket:', err);
    }
  }, [handleMessage]);

  // Keep the reconnect timer pointing at the current connect().
  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  useEffect(() => {
    connect();

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.onclose = null; // Unmounting is not a reason to reconnect
        wsRef.current.close();
        wsRef.current = null;
      }
      reconnectAttemptsRef.current = 0;
      setFailed(false);
    };
  }, [connect]);

  return (
    <FileSyncContext.Provider value={{ connected, reconnecting, failed, subscribe }}>
      {children}
    </FileSyncContext.Provider>
  );
}

function useFileSyncContext(): FileSyncContextValue | null {
  return useContext(FileSyncContext);
}

/**
 * Reports the shared connection's health. Returns a disconnected-but-not-failed
 * status outside a provider, so the docs-only build renders without one.
 */
export function useFileSyncStatus(): FileSyncStatus {
  const ctx = useFileSyncContext();
  if (!ctx) return { connected: false, reconnecting: false, failed: false };
  return { connected: ctx.connected, reconnecting: ctx.reconnecting, failed: ctx.failed };
}

/**
 * Calls `handler` for every file change the server reports. The handler is held in
 * a ref, so callers get the latest closure without needing to memoize it and
 * without churning the subscription.
 */
export function useFileSyncSubscription(handler: FileChangeHandler): void {
  const subscribe = useFileSyncContext()?.subscribe;
  const handlerRef = useRef(handler);

  useEffect(() => {
    handlerRef.current = handler;
  }, [handler]);

  // Depends on subscribe rather than the whole context value, which is a fresh
  // object on every connection-status change - resubscribing on each would be
  // pointless churn.
  useEffect(() => {
    if (!subscribe) return;
    return subscribe((change) => handlerRef.current(change));
  }, [subscribe]);
}
