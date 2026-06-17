package api

import "testing"

func TestNewServer_Addr(t *testing.T) {
	tests := []struct {
		host string
		port int
		want string
	}{
		{"127.0.0.1", 5260, "127.0.0.1:5260"},
		{"0.0.0.0", 5260, "0.0.0.0:5260"},
		{"localhost", 8080, "localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			handler := NewHandler(nil, &ProjectContext{})
			s := NewServer(handler, tt.host, tt.port, "")

			if got := s.Addr(); got != tt.want {
				t.Errorf("Addr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewServer_DefaultsToLoopback(t *testing.T) {
	// kan serve defaults --host to 127.0.0.1, not 0.0.0.0, so the API is not
	// reachable from other devices on the network unless explicitly opted
	// into via --host 0.0.0.0.
	handler := NewHandler(nil, &ProjectContext{})
	s := NewServer(handler, "127.0.0.1", 5260, "")

	if got, want := s.Addr(), "127.0.0.1:5260"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}
