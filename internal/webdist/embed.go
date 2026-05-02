// Package webdist embeds the built React+AntD-Pro SPA at compile time.
// The dist/ directory is populated by `pnpm build` from web/.
// A placeholder index.html is committed so the package compiles before any
// frontend build has run (e.g. on a fresh clone before `make build-web`).
package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the SPA filesystem rooted at "dist".
// Use with http.FileServer(http.FS(webdist.FS())) or with the SPA fallback handler.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		// Programmer error — the embed directive guarantees dist/ exists.
		panic("webdist: embedded dist subtree missing: " + err.Error())
	}
	return sub
}
