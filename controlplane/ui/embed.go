package ui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distEmbedFS embed.FS

// Dist returns an fs.FS sub-rooted at the dist folder.
func Dist() fs.FS {
	sub, err := fs.Sub(distEmbedFS, "dist")
	if err != nil {
		return distEmbedFS
	}
	return sub
}
