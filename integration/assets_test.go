//go:build linux && integration

package integration_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

type nopStorage struct{}

func (nopStorage) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(stringsReader("")), nil
}
func (nopStorage) Save(context.Context, string, io.Reader) error { return nil }
func (nopStorage) Stat(context.Context, string) (office.FileInfo, error) {
	return office.FileInfo{}, nil
}

type stringsReader string

func (s stringsReader) Read(p []byte) (int, error) {
	copy(p, []byte(s))
	return len(s), io.EOF
}

func assetDir(t *testing.T) string {
	t.Helper()
	dir := office.AssetDirFromEnv("assets")
	apiTpl := filepath.Join(dir, "web-apps", "apps", "api", "documents", "api.js.tpl")
	apiJs := filepath.Join(dir, "web-apps", "apps", "api", "documents", "api.js")
	if _, err := os.Stat(apiTpl); err != nil {
		if _, err2 := os.Stat(apiJs); err2 != nil {
			t.Skipf("Euro-Office assets not found in %s (run scripts/fetch-assets.sh): %v", dir, err)
		}
	}
	return dir
}

func TestServesAPIJS(t *testing.T) {
	dir := assetDir(t)
	protocol, err := office.ReadAssetVersion(dir)
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}

	srv, err := office.New(nopStorage{}, office.Options{
		AssetDir:        dir,
		BasePath:        "/office",
		ProtocolVersion: protocol,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/office/web-apps/apps/api/documents/api.js", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHealthWithAssets(t *testing.T) {
	dir := assetDir(t)
	srv, err := office.New(nopStorage{}, office.Options{AssetDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/office/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
