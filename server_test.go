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

	office "github.com/quantumx-apps/go-office"
	"github.com/quantumx-apps/go-office/config"
)

type nopStorage struct{}

func (nopStorage) Open(context.Context, string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("")), nil }
func (nopStorage) Save(context.Context, string, io.Reader) error       { return nil }
func (nopStorage) Stat(context.Context, string) (office.FileInfo, error) {
	return office.FileInfo{}, nil
}

func TestHealthEndpoint(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{BasePath: "/office"})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/office/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", rec.Body.String())
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
	}
}
