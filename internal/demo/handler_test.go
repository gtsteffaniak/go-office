package demo_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	office "github.com/quantumx-apps/go-office"
	"github.com/quantumx-apps/go-office/internal/demo"
	"github.com/quantumx-apps/go-office/internal/home"
)

type memStore struct {
	root string
}

func (m *memStore) Open(_ context.Context, rel string) (io.ReadCloser, error) {
	data, err := os.ReadFile(filepath.Join(m.root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}
func (m *memStore) Save(context.Context, string, io.Reader) error { return nil }
func (m *memStore) Stat(_ context.Context, rel string) (office.FileInfo, error) {
	full := filepath.Join(m.root, filepath.FromSlash(rel))
	fi, err := os.Stat(full)
	if err != nil {
		return office.FileInfo{}, err
	}
	return office.FileInfo{Name: fi.Name(), Path: rel, Size: fi.Size()}, nil
}

func TestDemoLandingAndConfig(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	sample := demo.DefaultSamplesDir + "/sample.doc"
	samplePath := filepath.Join(repoRoot, filepath.FromSlash(sample))
	if _, err := os.Stat(samplePath); err != nil {
		t.Skipf("repository sample not present: %v", err)
	}

	store := &memStore{root: repoRoot}
	srv, err := office.New(store, office.Options{BasePath: "/office"})
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.Attach(srv, store, demo.Options{
		PublicOrigin: "http://localhost:8080",
		DataRoot:     repoRoot,
		SamplesDir:   demo.DefaultSamplesDir,
		APIBasePath:  home.DefaultAPIBasePath,
	}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/office/demo/", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "sample.doc") {
		t.Fatalf("landing status=%d body=%q", rec.Code, body[:min(200, len(body))])
	}
	if !strings.Contains(body, "/office/demo/view?file=") {
		t.Fatal("expected viewer links on landing page")
	}

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/office/demo/view?file="+sample, nil))
	body = rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "DocsAPI.DocEditor") {
		t.Fatalf("viewer status=%d body=%q", rec.Code, body[:min(200, len(body))])
	}
	if !strings.Contains(body, "/api/office") || !strings.Contains(body, "/demo/config") {
		t.Fatal("expected API config URL in viewer page")
	}

	rec = httptest.NewRecorder()
	q := "/api/office/demo/config?file=" + sample
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, q, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("config status=%d body=%s", rec.Code, rec.Body.String())
	}
	var cfg map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	doc := cfg["document"].(map[string]any)
	if doc["fileType"] != "doc" {
		t.Fatalf("fileType = %v", doc["fileType"])
	}
	url := doc["url"].(string)
	if url == "" || !strings.Contains(url, "/api/office/demo/file/") {
		t.Fatalf("config document url: %+v", doc)
	}
}
