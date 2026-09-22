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
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
}

// Dir returns an http.Handler that serves files from root/subdir.
// Returns nil if the directory does not exist.
func Dir(root, subdir string) http.Handler {
	return DirWithPolicy(root, subdir, false)
}

// DirWithPolicy serves files from root/subdir. When immutableExtless is true,
// extensionless paths (e.g. font id binaries under /fonts/) also get long-lived cache headers.
func DirWithPolicy(root, subdir string, immutableExtless bool) http.Handler {
	dir := filepath.Join(root, subdir)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setContentType(w, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, ".html") || strings.HasSuffix(r.URL.Path, "/"):
			// Editor bootstrap pages embed the sdkjs patch revision in their RequireJS
			// config, so they must be revalidated on every load. A request for
			// `/…/index.html` is redirected by FileServer to the directory URL, which is
			// what actually serves the document, so both shapes must revalidate.
			w.Header().Set("Cache-Control", "no-cache")
		case cacheControlImmutable(r.URL.Path, immutableExtless):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fs.ServeHTTP(w, r)
	})
}

func cacheControlImmutable(path string, immutableExtless bool) bool {
	if strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".wasm") {
		return true
	}
	if !immutableExtless {
		return false
	}
	ext := filepath.Ext(path)
	if ext != "" {
		switch ext {
		case ".ttf", ".otf", ".woff", ".woff2":
			return true
		default:
			return false
		}
	}
	base := filepath.Base(strings.TrimSuffix(path, "/"))
	return base != "" && base != "."
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
