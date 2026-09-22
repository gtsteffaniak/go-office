package static

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDirWithPolicyCachesExtensionlessFonts(t *testing.T) {
	dir := t.TempDir()
	fontsDir := filepath.Join(dir, "fonts")
	if err := os.MkdirAll(fontsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fontsDir, "151"), []byte("font-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := DirWithPolicy(dir, "fonts", true)
	if h == nil {
		t.Fatal("handler is nil")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/151", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", cc)
	}
}

func TestDirDoesNotCacheExtensionlessByDefault(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "web-apps")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "noext"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := Dir(dir, "web-apps")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/noext", nil))
	if rec.Header().Get("Cache-Control") != "" {
		t.Fatalf("unexpected Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}

// TestDirRevalidatesEditorHTML guards the cache-busting path: the editor bootstrap page must
// never be served from a heuristic/immutable cache, because it carries the sdkjs RequireJS
// urlArgs revision that forces a refetch of patched bundles.
func TestDirRevalidatesEditorHTML(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "web-apps")
	main := filepath.Join(root, "apps", "spreadsheeteditor", "main")
	if err := os.MkdirAll(main, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := Dir(dir, "web-apps")
	// `/index.html` is redirected to the directory URL, which is the response that actually
	// carries the document, so assert on the directory form.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apps/spreadsheeteditor/main/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cc)
	}
}
