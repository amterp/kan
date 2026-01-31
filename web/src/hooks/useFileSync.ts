import { useEffect, useRef, useCallback, useState } from 'react';

// FileChange matches the Go FileChange struct
export interface FileChange {
  type: 'created' | 'modified' | 'deleted';
  kind: 'card' | 'board' | 'project' | 'unknown';
  board_name?: string;
  card_id?: string;
  path: string;
}

interface WebSocketMessage {
  type: string;
  data: FileChange | { message: string };
}

interface UseFileSyncOptions {
  onCardChange?: (change: FileChange) => void;
  onBoardChange?: (change: FileChange) => void;
  onProjectChange?: (change: FileChange) => void;
  boardFilter?: string; // Only receive changes for this board
  enabled?: boolean;
}

interface UseFileSyncResult {
  connected: boolean;
  reconnecting: boolean;
  failed: boolean; // True when max reconnect attempts reached
}

const RECONNECT_DELAY = 2000; // 2 seconds
const MAX_RECONNECT_ATTEMPTS = 10;

/**
 * useFileSync connects to the WebSocket endpoint for real-time file change notifications.
 * It automatically reconnects on disconnect and filters changes by board if specified.
 */
export function useFileSync(options: UseFileSyncOptions = {}): UseFileSyncResult {
  const {
    onCardChange,
    onBoardChange,
    onProjectChange,
    boardFilter,
    enabled = true,
  } = options;

  const [connected, setConnected] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [failed, setFailed] = useState(false);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Store callbacks in refs to avoid reconnecting when callbacks change
  const onCardChangeRef = useRef(onCardChange);
  const onBoardChangeRef = useRef(onBoardChange);
  const onProjectChangeRef = useRef(onProjectChange);
  const boardFilterRef = useRef(boardFilter);

  useEffect(() => {
    onCardChangeRef.current = onCardChange;
    onBoardChangeRef.current = onBoardChange;
    onProjectChangeRef.current = onProjectChange;
    boardFilterRef.current = boardFilter;
  }, [onCardChange, onBoardChange, onProjectChange, boardFilter]);

  const handleMessage = useCallback((event: MessageEvent) => {
    try {
      const message: WebSocketMessage = JSON.parse(event.data);

      if (message.type === 'connected') {
        return; // Initial connection message
      }

      if (message.type === 'file_change') {
        const change = message.data as FileChange;

        // Apply board filter if specified
        if (boardFilterRef.current && change.board_name && change.board_name !== boardFilterRef.current) {
          return;
        }

        // Dispatch to appropriate callback
        switch (change.kind) {
          case 'card':
            onCardChangeRef.current?.(change);
            break;
          case 'board':
            onBoardChangeRef.current?.(change);
            break;
          case 'project':
            onProjectChangeRef.current?.(change);
            break;
        }
      }
    } catch (err) {
      console.error('Failed to parse WebSocket message:', err);
    }
  }, []);

  const connect = useCallback(() => {
    if (!enabled) return;

    // Determine WebSocket URL based on current location
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/api/v1/ws`;

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

        // Attempt reconnect if not at max attempts
        if (enabled && reconnectAttemptsRef.current < MAX_RECONNECT_ATTEMPTS) {
          setReconnecting(true);
          reconnectAttemptsRef.current++;
          reconnectTimeoutRef.current = setTimeout(connect, RECONNECT_DELAY);
        } else if (reconnectAttemptsRef.current >= MAX_RECONNECT_ATTEMPTS) {
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
  }, [enabled, handleMessage]);

  useEffect(() => {
    if (enabled) {
      connect();
    }

    return () => {
      // Cleanup on unmount or when disabled
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      // Reset state when disabled (allows fresh start when re-enabled)
      reconnectAttemptsRef.current = 0;
      setFailed(false);
    };
  }, [enabled, connect]);

  return { connected, reconnecting, failed };
}
