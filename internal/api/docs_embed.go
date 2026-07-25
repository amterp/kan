package api

import (
	"embed"
	"io/fs"
)

//go:embed dist/docs/*.md
var docsFiles embed.FS

// DocsFS returns the embedded documentation markdown, rooted at the docs
// directory. Unlike the frontend embed this is not build-tagged: `kan docs`
// must work in dev builds too, and the markdown under dist/docs is a committed
// verbatim copy of web/src/docs.
func DocsFS() fs.FS {
	fsys, _ := fs.Sub(docsFiles, "dist/docs")
	return fsys
}
