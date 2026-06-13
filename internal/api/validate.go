package api

import (
	"net/http"
	"strings"
)

// isSafePathSegment reports whether s is safe to use as a single component
// (board name, card ID/alias, column name, comment ID) when building
// filesystem paths under the .kan directory. It rejects empty strings, ".",
// "..", and anything containing a path separator, which prevents these
// values from escaping the intended directory via filepath.Join.
func isSafePathSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	return !strings.ContainsAny(s, "/\\")
}

// validated wraps a handler, rejecting the request with 400 Bad Request if
// any of the named path parameters fail isSafePathSegment. This is enforced
// centrally for every route that takes a board/card/column/comment identifier
// from the URL, since those values are joined onto filesystem paths by the
// store layer (e.g. filepath.Join(boardsRoot, boardName, "config.toml")) and
// would otherwise allow path traversal (e.g. a board name of ".." reaching
// outside the boards directory).
func validated(next http.HandlerFunc, params ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, p := range params {
			if !isSafePathSegment(r.PathValue(p)) {
				BadRequest(w, "invalid "+p)
				return
			}
		}
		next(w, r)
	}
}
