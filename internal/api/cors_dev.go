//go:build dev

package api

import "net/http"

// devOrigins are the origins the Vite dev server may run on. Only these
// origins are allowed to make cross-origin requests in dev builds.
var devOrigins = map[string]bool{
	"http://localhost:5173": true,
	"http://127.0.0.1:5173": true,
}

// Cors allows the Vite dev server (run separately from the Go backend during
// development) to call the API across origins. Restricted to the known Vite
// dev server origins rather than "*", so dev builds don't reintroduce the
// open-CORS issue described in cors.go.
func Cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if devOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
