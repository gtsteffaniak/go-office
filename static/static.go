package static

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Dir returns an http.Handler that serves files from root/subdir.
// Returns nil if the directory does not exist.
func Dir(root, subdir string) http.Handler {
	dir := filepath.Join(root, subdir)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Immutable caching for versioned editor assets.
		if strings.HasSuffix(r.URL.Path, ".js") ||
			strings.HasSuffix(r.URL.Path, ".css") ||
			strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fs.ServeHTTP(w, r)
	})
}
