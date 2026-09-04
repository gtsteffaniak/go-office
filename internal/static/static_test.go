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
