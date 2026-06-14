//go:build dev

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCors_AllowsViteDevOrigin(t *testing.T) {
	called := false
	handler := Cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/boards", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected wrapped handler to be called")
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:5173")
	}
}

func TestCors_RejectsUnknownOrigin(t *testing.T) {
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
		t.Errorf("expected no Access-Control-Allow-Origin header for unknown origin, got %q", got)
	}
}

func TestCors_OptionsPreflightHandledWithoutCallingNext(t *testing.T) {
	called := false
	handler := Cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("OPTIONS", "/api/v1/boards", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("expected OPTIONS preflight to be handled without calling next")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for preflight, got %d", w.Code)
	}
}

func TestCheckOrigin_ViteDevOriginAllowedInDevBuild(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/ws", nil)
	req.Host = "127.0.0.1:5260"
	req.Header.Set("Origin", "http://localhost:5173")

	if !checkOrigin(req) {
		t.Error("expected Vite dev server origin to be allowed in a dev build")
	}
}
