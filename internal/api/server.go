package api

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Server wraps the HTTP server for the web frontend.
type Server struct {
	httpServer *http.Server
}

// NewServer creates a new server with the given handler and port.
func NewServer(handler *Handler, port int) *Server {
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	wrapped := Logging(Cors(mux))

	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      wrapped,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
	}
}

// Start begins listening for HTTP requests. Blocks until shutdown.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}
