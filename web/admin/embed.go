package webadmin

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns the built admin SPA as a filesystem rooted at dist/.
// In dev, dist/ may contain only a stub index.html — real assets are
// produced by `npm run build` (see Makefile build-web).
func Dist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
