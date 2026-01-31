package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWebSocketHub_AddRemoveClient(t *testing.T) {
	hub := NewWebSocketHub()

	client := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 10),
	}

	hub.addClient(client)
	if hub.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", hub.ClientCount())
	}

	hub.removeClient(client)
	if hub.ClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestWebSocketHub_RemoveClientClosesChannel(t *testing.T) {
	hub := NewWebSocketHub()

	client := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 10),
	}

	hub.addClient(client)
	hub.removeClient(client)

	// Verify channel is closed by checking if receive returns immediately
	select {
	case _, ok := <-client.send:
		if ok {
			t.Error("Channel should be closed")
		}
	default:
		t.Error("Channel should be closed and readable")
	}
}

func TestWebSocketHub_RemoveClientIdempotent(t *testing.T) {
	hub := NewWebSocketHub()

	client := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 10),
	}

	hub.addClient(client)
	hub.removeClient(client)
	hub.removeClient(client) // Should not panic

	if hub.ClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestWebSocketHub_Broadcast(t *testing.T) {
	hub := NewWebSocketHub()

	client1 := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 10),
	}
	client2 := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 10),
	}

	hub.addClient(client1)
	hub.addClient(client2)

	testData := []byte(`{"test": "data"}`)
	hub.broadcast(testData)

	// Both clients should receive the message
	select {
	case msg := <-client1.send:
		if string(msg) != string(testData) {
			t.Errorf("Client 1 got %q, want %q", msg, testData)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Client 1 did not receive message")
	}

	select {
	case msg := <-client2.send:
		if string(msg) != string(testData) {
			t.Errorf("Client 2 got %q, want %q", msg, testData)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Client 2 did not receive message")
	}
}

func TestWebSocketHub_BroadcastToRemovedClient(t *testing.T) {
	hub := NewWebSocketHub()

	client := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 10),
	}

	hub.addClient(client)
	hub.removeClient(client)

	// This should not panic even though client's channel is closed
	hub.broadcast([]byte(`{"test": "data"}`))
}

func TestWebSocketHub_TrySendRecovery(t *testing.T) {
	hub := NewWebSocketHub()

	client := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 10),
	}

	// Close the channel to simulate a removed client
	close(client.send)

	// trySend should recover from the panic and not crash
	hub.trySend(client, []byte(`test`))
	// If we get here without panic, the test passes
}

func TestWebSocketHub_OnFileChange(t *testing.T) {
	hub := NewWebSocketHub()

	client := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 10),
	}
	hub.addClient(client)

	change := FileChange{
		Type:      FileChangeModified,
		Kind:      FileChangeKindCard,
		BoardName: "main",
		CardID:    "abc123",
		Path:      "boards/main/cards/abc123.json",
	}

	hub.OnFileChange(change)

	select {
	case msg := <-client.send:
		var received WebSocketMessage
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}
		if received.Type != "file_change" {
			t.Errorf("Type = %q, want %q", received.Type, "file_change")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive file change message")
	}
}

func TestWebSocketHub_BroadcastFullBuffer(t *testing.T) {
	hub := NewWebSocketHub()

	// Create a client with a full buffer
	client := &WebSocketClient{
		hub:  hub,
		send: make(chan []byte, 1), // Small buffer
	}
	hub.addClient(client)

	// Fill the buffer
	client.send <- []byte("first")

	// This broadcast should trigger removal due to full buffer
	hub.broadcast([]byte("second"))

	// Client should be removed
	if hub.ClientCount() != 0 {
		t.Errorf("Expected client to be removed due to full buffer, got %d clients", hub.ClientCount())
	}
}
