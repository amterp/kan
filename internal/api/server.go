package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Server wraps the HTTP server for the web frontend.
type Server struct {
	httpServer *http.Server
	watcher    *FileWatcher
	wsHub      *WebSocketHub
	watcherMu  sync.Mutex // Protects watcher during project switches
}

// NewServer creates a new server with the given handler, port, and project root.
// If projectRoot is empty, file watching is disabled.
func NewServer(handler *Handler, port int, projectRoot string) *Server {
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Set up file watching and WebSocket if we have a project root
	var watcher *FileWatcher
	var wsHub *WebSocketHub

	if projectRoot != "" {
		wsHub = NewWebSocketHub()
		mux.HandleFunc("GET /api/v1/ws", wsHub.ServeWS)

		var err error
		watcher, err = NewFileWatcher(projectRoot)
		if err != nil {
			log.Printf("Warning: failed to create file watcher: %v", err)
		} else {
			watcher.Subscribe(wsHub)
		}
	}

	wrapped := Logging(Cors(mux))

	s := &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      wrapped,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		watcher: watcher,
		wsHub:   wsHub,
	}

	// Set up callback to switch watcher when project changes
	handler.SetOnProjectSwitch(s.switchWatcher)

	return s
}

// Start begins listening for HTTP requests. Blocks until shutdown.
func (s *Server) Start() error {
	// Start file watcher if available
	if s.watcher != nil {
		if err := s.watcher.Start(); err != nil {
			log.Printf("Warning: failed to start file watcher: %v", err)
		}
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop file watcher
	if s.watcher != nil {
		s.watcher.Stop()
	}

	return s.httpServer.Shutdown(ctx)
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// switchWatcher stops the current file watcher and starts a new one for the given project.
func (s *Server) switchWatcher(newProjectRoot string) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()

	// Stop old watcher
	if s.watcher != nil {
		if err := s.watcher.Stop(); err != nil {
			log.Printf("Warning: failed to stop file watcher: %v", err)
		}
		s.watcher = nil
	}

	// Skip if no WebSocket hub (file watching disabled)
	if s.wsHub == nil {
		return
	}

	// Create and start new watcher
	watcher, err := NewFileWatcher(newProjectRoot)
	if err != nil {
		log.Printf("Warning: failed to create file watcher for %s: %v", newProjectRoot, err)
		return
	}

	watcher.Subscribe(s.wsHub)
	if err := watcher.Start(); err != nil {
		log.Printf("Warning: failed to start file watcher: %v", err)
		return
	}

	s.watcher = watcher
	log.Printf("File watcher switched to: %s", newProjectRoot)
}
