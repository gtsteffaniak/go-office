package static

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

// compressibleExts are asset types worth compressing. The editor ships large uncompressed
// JS/CSS/XML: measured on the bundled Euro-Office tree, sdk-all-min.js is 2.4 MB -> 472 KB,
// app.js 2.5 MB -> 497 KB, app.css 572 KB -> 72 KB (81-88% smaller). With Playwright running
// one browser per worker and each fetching these independently, sending them uncompressed is
// the single largest avoidable cost of an editor boot in CI.
var compressibleExts = map[string]bool{
	".js":   true,
	".mjs":  true,
	".css":  true,
	".json": true,
	".svg":  true,
	".xml":  true,
	".html": true,
	".htm":  true,
	".txt":  true,
	".map":  true,
	".woff": true,
}

// alreadyCompressedExts are formats where gzip wastes CPU for no gain.
var alreadyCompressedExts = map[string]bool{
	".wasm": true, // typically already a compact binary format
	".gz":   true,
	".zip":  true,
	".br":   true,
}

// minCompressSize avoids gzip overhead on tiny files where the framing cost dominates.
const minCompressSize = 1024

// gzipCacheEntry is a rendered gzip body plus the validators of the source file.
type gzipCacheEntry struct {
	body    []byte
	modTime int64
	size    int64
}

// gzipCache memoises compressed bodies. The editor loads ~10-20 MB of assets per page and
// Playwright runs many pages per suite, so recompressing on every request would burn the CPU
// we are trying to save. Entries are keyed by path and invalidated when the source file
// changes (mtime/size), which keeps asset hot-reload working during local development.
type gzipCache struct {
	mu      sync.RWMutex
	entries map[string]gzipCacheEntry
	// maxBytes bounds total retained compressed bytes (default 64 MiB).
	maxBytes int
	curBytes int
}

func newGzipCache() *gzipCache {
	return &gzipCache{entries: make(map[string]gzipCacheEntry), maxBytes: 64 << 20}
}

func (c *gzipCache) get(key string, modTime, size int64) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || e.modTime != modTime || e.size != size {
		return nil, false
	}
	return e.body, true
}

func (c *gzipCache) put(key string, modTime, size int64, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.entries[key]; ok {
		c.curBytes -= len(prev.body)
	}
	// Simple bound: stop adding once over budget rather than running an LRU.
	if c.curBytes+len(body) > c.maxBytes {
		return
	}
	c.entries[key] = gzipCacheEntry{body: body, modTime: modTime, size: size}
	c.curBytes += len(body)
}

// acceptsGzip reports whether the client advertised gzip support.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		enc := strings.TrimSpace(part)
		if i := strings.IndexByte(enc, ';'); i >= 0 {
			enc = strings.TrimSpace(enc[:i])
		}
		if strings.EqualFold(enc, "gzip") {
			return true
		}
	}
	return false
}

// shouldCompress reports whether path is worth compressing and is large enough to matter.
func shouldCompress(path string, size int64) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if alreadyCompressedExts[ext] {
		return false
	}
	if !compressibleExts[ext] {
		return false
	}
	return size >= minCompressSize
}

// gzipBytes compresses up to size bytes of r into memory.
func gzipBytes(r io.Reader, size int64) ([]byte, error) {
	var buf bytes.Buffer
	// Pre-size for a typical ~20% ratio to avoid repeated growth.
	if size > 0 {
		buf.Grow(int(size/4) + 64)
	}
	zw := gzip.NewWriter(&buf)
	if _, err := io.Copy(zw, io.LimitReader(r, size)); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
