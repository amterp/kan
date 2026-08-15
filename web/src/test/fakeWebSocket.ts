import type { FileChange } from '../api/types';

type Listener = ((event: unknown) => void) | null;

/**
 * A stand-in for the browser WebSocket, which jsdom does not provide.
 *
 * The socket is the only outside-world edge in the file sync path, so faking
 * exactly it leaves the provider, the hooks and the reducers under test running
 * for real. Tests drive the server side through `open`, `emitChange` and
 * `serverClose` rather than waiting on anything.
 */
export class FakeWebSocket {
  static instances: FakeWebSocket[] = [];

  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;

  readonly url: string;
  readyState: number = FakeWebSocket.CONNECTING;

  onopen: Listener = null;
  onclose: Listener = null;
  onerror: Listener = null;
  onmessage: Listener = null;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  /** Called by the code under test. */
  close(): void {
    if (this.readyState === FakeWebSocket.CLOSED) return;
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.({});
  }

  send(): void {
    // The client never sends; present only to satisfy the interface.
  }

  // --- test drivers ---

  /** The server accepted the connection. */
  open(): void {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.({});
  }

  /** The server reported a file change. */
  emitChange(change: FileChange): void {
    this.deliver(JSON.stringify({ type: 'file_change', data: change }));
  }

  /** The greeting the real server sends on connect. */
  emitGreeting(): void {
    this.deliver(JSON.stringify({ type: 'connected', data: { message: 'File sync enabled' } }));
  }

  /** Raw frame, for malformed-payload cases. */
  deliver(data: string): void {
    this.onmessage?.({ data });
  }

  /** The connection dropped from the server's end. */
  serverClose(): void {
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.({});
  }

  /** The most recently constructed socket, which is the live one. */
  static get latest(): FakeWebSocket {
    const socket = FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
    if (!socket) throw new Error('no WebSocket was created');
    return socket;
  }
}

/** Installs the fake and clears any sockets left over from a previous test. */
export function installFakeWebSocket(): void {
  FakeWebSocket.instances = [];
  (globalThis as { WebSocket?: unknown }).WebSocket = FakeWebSocket;
}
