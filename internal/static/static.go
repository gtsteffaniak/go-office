package static

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
//
// Responses are gzip-encoded on the fly when the client accepts it and the file is a
// compressible type. The editor bundle ships uncompressed, and Playwright gives every worker
// its own browser cache, so each worker otherwise downloads ~5 MB of highly compressible
// JS/CSS per editor boot.
func DirWithPolicy(root, subdir string, immutableExtless bool) http.Handler {
	dir := filepath.Join(root, subdir)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	fs := http.FileServer(http.Dir(dir))
	cache := newGzipCache()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		setContentType(w, path)
		if cacheControlImmutable(path, immutableExtless) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		// Vary is required so caches never serve a gzipped body to a client that did not
		// ask for gzip.
		if shouldCompress(path, 1) || compressibleExts[strings.ToLower(filepath.Ext(path))] {
			w.Header().Add("Vary", "Accept-Encoding")
		}
		if r.Method == http.MethodGet && acceptsGzip(r) {
			if served := serveGzipAsset(w, r, dir, path, cache); served {
				return
			}
		}
		fs.ServeHTTP(w, r)
	})
}

// serveGzipAsset writes a gzip response for a static file. It returns false when the request
// should fall through to the plain file server (not found, not worth compressing, or the
// client is not to be compressed).
func serveGzipAsset(w http.ResponseWriter, r *http.Request, dir, urlPath string, cache *gzipCache) bool {
	clean := filepath.Clean("/" + strings.TrimPrefix(urlPath, "/"))
	if strings.Contains(clean, "..") {
		return false
	}
	full := filepath.Join(dir, filepath.FromSlash(clean))
	st, err := os.Stat(full)
	if err != nil || st.IsDir() || !st.Mode().IsRegular() {
		return false
	}
	if !shouldCompress(urlPath, st.Size()) {
		return false
	}
	if body, ok := cache.get(full, st.ModTime().UnixNano(), st.Size()); ok {
		writeGzipBytes(w, body)
		return true
	}
	f, err := os.Open(full)
	if err != nil {
		return false
	}
	defer f.Close()
	body, err := gzipBytes(f, st.Size())
	if err != nil {
		return false
	}
	cache.put(full, st.ModTime().UnixNano(), st.Size(), body)
	writeGzipBytes(w, body)
	return true
}

func writeGzipBytes(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
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
