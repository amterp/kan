package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local development
	},
}

// WebSocketHub manages WebSocket connections and broadcasts file changes.
type WebSocketHub struct {
	mu      sync.RWMutex
	clients map[*WebSocketClient]bool
}

// WebSocketClient represents a connected WebSocket client.
type WebSocketClient struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan []byte
}

// WebSocketMessage is the JSON message sent to clients.
type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// NewWebSocketHub creates a new WebSocket hub.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients: make(map[*WebSocketClient]bool),
	}
}

// OnFileChange implements FileWatcherSubscriber.
func (h *WebSocketHub) OnFileChange(change FileChange) {
	msg := WebSocketMessage{
		Type: "file_change",
		Data: change,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal file change: %v", err)
		return
	}

	h.broadcast(data)
}

// broadcast sends a message to all connected clients.
func (h *WebSocketHub) broadcast(data []byte) {
	h.mu.RLock()
	clients := make([]*WebSocketClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		h.trySend(client, data)
	}
}

// trySend attempts to send data to a client, handling the case where
// the client's channel was closed between snapshot and send.
func (h *WebSocketHub) trySend(client *WebSocketClient, data []byte) {
	defer func() {
		if r := recover(); r != nil {
			// Channel was closed by removeClient - client already cleaned up
		}
	}()

	select {
	case client.send <- data:
	default:
		// Client buffer full, close it
		h.removeClient(client)
	}
}

func (h *WebSocketHub) addClient(client *WebSocketClient) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
}

func (h *WebSocketHub) removeClient(client *WebSocketClient) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	h.mu.Unlock()
}

// ServeWS handles WebSocket connection requests.
func (h *WebSocketHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &WebSocketClient{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.addClient(client)

	// Start read/write goroutines
	go client.writePump()
	go client.readPump()

	// Send initial connection message
	welcome := WebSocketMessage{
		Type: "connected",
		Data: map[string]interface{}{
			"message": "File sync enabled",
		},
	}
	if data, err := json.Marshal(welcome); err == nil {
		client.send <- data
	}
}

// readPump reads messages from the WebSocket connection.
// We don't expect client messages, but we need to read to detect disconnects.
func (c *WebSocketClient) readPump() {
	defer func() {
		// Only call removeClient here - closing send channel signals writePump to exit
		// writePump is responsible for closing the connection
		c.hub.removeClient(c)
	}()

	c.conn.SetReadLimit(512) // Small limit since we don't expect large messages
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(30 * time.Second) // Ping interval
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send each message as its own WebSocket frame (not batched)
			// This ensures the frontend receives valid JSON for each message
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Send any queued messages as separate frames
			n := len(c.send)
			for i := 0; i < n; i++ {
				queuedMsg := <-c.send
				if err := c.conn.WriteMessage(websocket.TextMessage, queuedMsg); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
