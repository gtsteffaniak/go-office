package office_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/pkg/config"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

type nopStorage struct{}

func (nopStorage) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (nopStorage) Save(context.Context, string, io.Reader) error { return nil }
func (nopStorage) Stat(context.Context, string) (office.FileInfo, error) {
	return office.FileInfo{}, nil
}

func TestHealthEndpoint(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"sessions":`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"cacheDirs":`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestPluginsJSONEndpoint(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/plugins.json", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "[]" {
		t.Fatalf("plugins.json = %d %q", rec.Code, rec.Body.String())
	}
}

func TestHealthCheckEndpoint(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "true" {
		t.Fatalf("healthcheck = %d %q", rec.Code, rec.Body.String())
	}
}

func TestBuildEditorConfigRequiresKey(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.BuildEditorConfig(context.Background(), config.EditorRequest{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestDocumentServerURL(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{BasePath: "/myapp/office"})
	if err != nil {
		t.Fatal(err)
	}
	got := srv.DocumentServerURL("http://localhost:8080")
	if got != "http://localhost:8080/myapp/office/" {
		t.Fatalf("url = %q", got)
	}
}

func TestMirrorsRootAssets(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"web-apps/apps", "sdkjs/common"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "sdkjs", "common", "device_scale.js"), []byte("// ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := office.New(nopStorage{}, office.Options{
		AssetDir: dir,
		BasePath: "/office",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/sdkjs/common/device_scale.js", "/office/sdkjs/common/device_scale.js"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
			t.Fatalf("%s content-type = %q", path, rec.Header().Get("Content-Type"))
		}
	}
}

func TestDownloadFile(t *testing.T) {
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 test"))
	}))
	defer fileSrv.Close()

	srv, err := office.New(nopStorage{}, office.Options{BasePath: "/office"})
	if err != nil {
		t.Fatal(err)
	}
	const key = "abc123"
	if _, err := srv.BuildEditorConfig(context.Background(), config.EditorRequest{
		DocumentKey: key,
		FileType:    "pdf",
		DocumentURL: fileSrv.URL + "/sample.pdf",
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/downloadfile/"+key, strings.NewReader(`{"url":"`+fileSrv.URL+`/sample.pdf"}`))
	req.Header.Set("Range", "bytes=0-3")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "%PDF" {
		t.Fatalf("range body = %q", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/downloadfile/"+key, nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "pdf") {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "%PDF") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestServesFonts(t *testing.T) {
	dir := t.TempDir()
	fontsDir := filepath.Join(dir, "fonts")
	if err := os.MkdirAll(fontsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fontsDir, "151"), []byte("font-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := office.New(nopStorage{}, office.Options{
		AssetDir: dir,
		BasePath: "/office",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/office/fonts/151", "/fonts/151"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", path, rec.Code, rec.Body.String())
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
			t.Fatalf("%s Cache-Control = %q", path, cc)
		}
	}
}

func TestEditorBinIsNotBrowserCached(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache", "doc-key")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "Editor.bin"), []byte("editor-bin"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := office.New(nopStorage{}, office.Options{AssetDir: dir})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/cache/files/doc-key/Editor.bin", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Editor.bin Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
}

func TestSessionResetEndpoint(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{BasePath: "/office"})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/office/session/reset?key=doc-key", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/office/session/reset?key=doc-key", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/office/session/reset", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing key status = %d, want 400", rec.Code)
	}
}

func TestBuildEditorConfigClearsCoauthoringSession(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.BuildEditorConfig(context.Background(), config.EditorRequest{
		DocumentKey: "clear-me",
		Title:       "t",
		FileType:    "docx",
		DocumentURL: "http://example/doc",
		CallbackURL: "http://example/cb",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Second config for same key must not error; coauthoring session was cleared.
	_, err = srv.BuildEditorConfig(context.Background(), config.EditorRequest{
		DocumentKey: "clear-me",
		Title:       "t",
		FileType:    "docx",
		DocumentURL: "http://example/doc",
		CallbackURL: "http://example/cb",
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Without a JWT secret the editor config must still carry a signed token. sdkjs assigns the
// top-level config token over the document token, so omitting it makes docInfo.get_Token()
// undefined and the editor substitutes its hardcoded placeholder ("fghhfgsjdgfjs") in the
// coauthoring auth packet. A server with verification disabled accepts that, which is why
// the defect only shows up once a secret is configured — as every session being rejected.
func TestBuildEditorConfigAlwaysSignsToken(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := srv.BuildEditorConfig(context.Background(), config.EditorRequest{
		DocumentKey: "k",
		Title:       "t.docx",
		FileType:    "docx",
		DocumentURL: "http://example/doc",
		CallbackURL: "http://example/cb",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _ := cfg["token"].(string)
	if token == "" {
		t.Fatal("top-level token missing; sdkjs would use its hardcoded placeholder")
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("token %q is not a JWT; sdkjs forwards it verbatim into the auth packet", token)
	}
	doc, _ := cfg["document"].(map[string]any)
	if doc == nil {
		t.Fatalf("document = %T", cfg["document"])
	}
	if got, _ := doc["token"].(string); got != token {
		t.Fatalf("document.token = %q, want the signed token %q", got, token)
	}

	// The unverified token must not be stable across processes in a way that suggests it is
	// a usable credential: two independent servers sign with different keys.
	other, err := office.New(nopStorage{}, office.Options{})
	if err != nil {
		t.Fatal(err)
	}
	cfg2, err := other.BuildEditorConfig(context.Background(), config.EditorRequest{
		DocumentKey: "k",
		Title:       "t.docx",
		FileType:    "docx",
		DocumentURL: "http://example/doc",
		CallbackURL: "http://example/cb",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token2, _ := cfg2["token"].(string); token2 == token {
		t.Fatal("independent servers produced the same unverified token; signing key is not per-process")
	}
}
