//go:build !dev

package api

import "net/http"

// devOrigins is empty in production builds; see cors_dev.go.
var devOrigins = map[string]bool{}

// Cors is a no-op in production builds.
//
// The release binary serves the frontend and the API from the same origin
// (the frontend is embedded into the binary), so no cross-origin requests
// are expected. Previously this middleware sent
// "Access-Control-Allow-Origin: *" for every response, which let any website
// the user happened to have open in their browser make authenticated-by-default
// requests (including state-changing POST/PUT/PATCH/DELETE, since the
// permissive headers also satisfied CORS preflight) to this local server.
// Not setting these headers causes browsers to block cross-origin requests
// from arbitrary sites, while same-origin requests from the embedded
// frontend are unaffected.
func Cors(next http.Handler) http.Handler {
	return next
}
