//go:build !dev

package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var staticFiles embed.FS

// StaticHandler returns a handler that serves the embedded frontend files.
func (h *Handler) StaticHandler() http.Handler {
	fsys, _ := fs.Sub(staticFiles, "dist")
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve index.html for SPA routes (paths without file extensions)
		path := r.URL.Path
		if path != "/" && !strings.Contains(path, ".") {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
