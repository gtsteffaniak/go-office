package static

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDirDictionariesGzip(t *testing.T) {
	dir := t.TempDir()
	lang := filepath.Join(dir, "dictionaries", "en_US")
	if err := os.MkdirAll(lang, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("hunspell-dictionary-bytes")
	if err := os.WriteFile(filepath.Join(lang, "en_US.dic"), payload, 0o644); err != nil {
		t.Fatal(err)
	}

	h := DirDictionaries(dir)
	if h == nil {
		t.Fatal("handler is nil")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/en_US/en_US.dic", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q", rec.Header().Get("Content-Encoding"))
	}
	if cc := rec.Header().Get("Cache-Control"); cc != dictionaryCacheControl {
		t.Fatalf("Cache-Control = %q", cc)
	}

	gz, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	got, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("body = %q", got)
	}
}

func TestDirDictionariesNoImmutableCache(t *testing.T) {
	dir := t.TempDir()
	lang := filepath.Join(dir, "dictionaries", "en_US")
	if err := os.MkdirAll(lang, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lang, "en_US.aff"), []byte("aff"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := DirDictionaries(dir)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/en_US/en_US.aff", nil))
	if rec.Header().Get("Cache-Control") == "public, max-age=31536000, immutable" {
		t.Fatal("dictionary files must not be immutable")
	}
}
