// Package webdist embeds the built frontend assets so the Go server can serve
// them without any separate static-file deployment. The `dist/` directory is
// the Vite build output from `../web/`.
package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var raw embed.FS

// FS returns the filesystem rooted at the dist directory, or nil if the
// directory has no real files yet (only the placeholder).
func FS() fs.FS {
	sub, err := fs.Sub(raw, "dist")
	if err != nil {
		return nil
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.Name() != "placeholder.txt" {
			return sub
		}
	}
	return nil
}
