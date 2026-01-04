//go:build dev

package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// StaticHandler returns a handler that proxies to the Vite dev server.
func (h *Handler) StaticHandler() http.Handler {
	target, _ := url.Parse("http://localhost:5173")
	proxy := httputil.NewSingleHostReverseProxy(target)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})
}
