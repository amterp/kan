//go:build !dev

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCors_NoOp(t *testing.T) {
	called := false
	handler := Cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/boards", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected wrapped handler to be called")
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Methods header, got %q", got)
	}
}

func TestCors_NoOp_OptionsStillReachesHandler(t *testing.T) {
	called := false
	handler := Cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/api/v1/boards", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected OPTIONS requests to pass through to the next handler")
	}
}

func TestDevOrigins_EmptyInProductionBuild(t *testing.T) {
	if len(devOrigins) != 0 {
		t.Errorf("expected devOrigins to be empty in a production build, got %v", devOrigins)
	}
}

func TestCheckOrigin_ViteDevOriginRejectedInProductionBuild(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/ws", nil)
	req.Host = "127.0.0.1:5260"
	req.Header.Set("Origin", "http://localhost:5173")

	if checkOrigin(req) {
		t.Error("expected Vite dev server origin to be rejected in a production build")
	}
}
