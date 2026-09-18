package static

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newAssetDir creates an asset tree with one compressible and one binary file.
func newAssetDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "sdkjs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Highly compressible, like the real editor bundles.
	js := strings.Repeat("window.__editor_bundle = function () { return 1; };\n", 400)
	if err := os.WriteFile(filepath.Join(dir, "bundle.js"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	// Binary assets must not be gzipped.
	if err := os.WriteFile(filepath.Join(dir, "font.wasm"), bytes.Repeat([]byte{0x00, 0x61, 0x73, 0x6d}, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	// Tiny files are not worth the gzip framing cost.
	if err := os.WriteFile(filepath.Join(dir, "tiny.js"), []byte("x=1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestServesGzipWhenAccepted is the core of the asset-weight reduction: the editor bundles
// ship uncompressed and every Playwright worker has its own browser cache, so the server must
// compress them on the wire.
func TestServesGzipWhenAccepted(t *testing.T) {
	root := newAssetDir(t)
	h := DirWithPolicy(root, "sdkjs", false)
	if h == nil {
		t.Fatal("handler is nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/bundle.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("Vary = %q, want to include Accept-Encoding", got)
	}

	zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	disk, err := os.ReadFile(filepath.Join(root, "sdkjs", "bundle.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, disk) {
		t.Fatal("gunzipped body does not match the file on disk")
	}
	if len(rec.Body.Bytes()) >= len(disk) {
		t.Fatalf("compressed size %d should be smaller than raw %d", rec.Body.Len(), len(disk))
	}
}

// TestNoGzipWhenNotAccepted guarantees we never send a gzipped body to a client that did not
// ask for it, which would corrupt the response.
func TestNoGzipWhenNotAccepted(t *testing.T) {
	root := newAssetDir(t)
	h := DirWithPolicy(root, "sdkjs", false)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bundle.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty when gzip not accepted", got)
	}
	disk, err := os.ReadFile(filepath.Join(root, "sdkjs", "bundle.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rec.Body.Bytes(), disk) {
		t.Fatal("unencoded body does not match the file on disk")
	}
}

// TestBinaryAssetsNotCompressed ensures already-compact binary formats are passed through.
func TestBinaryAssetsNotCompressed(t *testing.T) {
	root := newAssetDir(t)
	h := DirWithPolicy(root, "sdkjs", false)

	req := httptest.NewRequest(http.MethodGet, "/font.wasm", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for .wasm", got)
	}
}

// TestTinyAssetsNotCompressed avoids gzip framing overhead on small files.
func TestTinyAssetsNotCompressed(t *testing.T) {
	root := newAssetDir(t)
	h := DirWithPolicy(root, "sdkjs", false)

	req := httptest.NewRequest(http.MethodGet, "/tiny.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty for a sub-threshold file", got)
	}
}

// TestGzipCacheInvalidatesOnChange keeps local asset hot-reload working: editing a file must
// not serve a stale compressed body.
func TestGzipCacheInvalidatesOnChange(t *testing.T) {
	root := newAssetDir(t)
	h := DirWithPolicy(root, "sdkjs", false)
	path := filepath.Join(root, "sdkjs", "bundle.js")

	fetch := func() string {
		req := httptest.NewRequest(http.MethodGet, "/bundle.js", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatalf("body is not valid gzip: %v", err)
		}
		body, _ := io.ReadAll(zr)
		return string(body)
	}

	first := fetch()
	if !strings.Contains(first, "return 1") {
		t.Fatal("unexpected initial body")
	}

	updated := strings.Repeat("window.__editor_bundle = function () { return 22; };\n", 400)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure the mtime changes even on coarse-grained filesystems.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	second := fetch()
	if !strings.Contains(second, "return 22") {
		t.Fatal("compressed cache served a stale body after the file changed")
	}
}

// TestMissingFileFallsThrough ensures the gzip path does not turn a 404 into a 200.
func TestMissingFileFallsThrough(t *testing.T) {
	root := newAssetDir(t)
	h := DirWithPolicy(root, "sdkjs", false)

	req := httptest.NewRequest(http.MethodGet, "/nope.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestPathTraversalRejected ensures the gzip path cannot escape the asset root.
func TestPathTraversalRejected(t *testing.T) {
	root := newAssetDir(t)
	h := DirWithPolicy(root, "sdkjs", false)

	req := httptest.NewRequest(http.MethodGet, "/../../etc/passwd", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatal("path traversal must not be served")
	}
}
