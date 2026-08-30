package static

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	// Go's default types.db often lacks .wasm; ensure editor assets get executable types.
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".mjs", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".json", "application/json")
}

// Dir returns an http.Handler that serves files from root/subdir.
// Returns nil if the directory does not exist.
func Dir(root, subdir string) http.Handler {
	dir := filepath.Join(root, subdir)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setContentType(w, r.URL.Path)
		// Immutable caching for versioned editor assets.
		if strings.HasSuffix(r.URL.Path, ".js") ||
			strings.HasSuffix(r.URL.Path, ".css") ||
			strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fs.ServeHTTP(w, r)
	})
}

func setContentType(w http.ResponseWriter, name string) {
	ext := filepath.Ext(name)
	if ext == "" {
		return
	}
	if ctype := mime.TypeByExtension(ext); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
}
