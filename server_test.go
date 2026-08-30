package office_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
	srv, err := office.New(nopStorage{}, office.Options{BasePath: "/api/office"})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/office/health", nil)
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
	srv, err := office.New(nopStorage{}, office.Options{BasePath: "/myapp/api/office"})
	if err != nil {
		t.Fatal(err)
	}
	got := srv.DocumentServerURL("http://localhost:8080")
	if got != "http://localhost:8080/myapp/api/office/" {
		t.Fatalf("url = %q", got)
	}
}
