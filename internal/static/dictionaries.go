package static

import (
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const dictionaryCacheControl = "public, max-age=86400"

// DirDictionaries serves Hunspell dictionary files with gzip when the client accepts it.
// Dictionary URLs are not marked immutable because asset upgrades replace bytes at the same path.
func DirDictionaries(root string) http.Handler {
	dir := filepath.Join(root, "dictionaries")
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
		if name == "." || name == "/" || strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(dir, name)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		setContentType(w, name)
		w.Header().Set("Cache-Control", dictionaryCacheControl)
		if wantsGzip(r) && isDictionaryCompressible(name) {
			serveGzipFile(w, path, st)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		http.ServeContent(w, r, name, st.ModTime(), f)
	})
}

func wantsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

func isDictionaryCompressible(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".aff" || ext == ".dic"
}

func serveGzipFile(w http.ResponseWriter, path string, st os.FileInfo) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Vary", "Accept-Encoding")
	w.WriteHeader(http.StatusOK)

	gz := gzip.NewWriter(w)
	if _, err := io.Copy(gz, f); err != nil {
		return
	}
	_ = gz.Close()
}
