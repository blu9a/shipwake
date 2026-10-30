// Package site holds the built web app, embedded into the binary.
package site

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// Web returns the web app's files. It is empty until the web app has been built into dist/.
func Web() fs.FS {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
