package office_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/quantumx-apps/go-office/internal/session"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestFlushDocumentNoOpForceSkipsCallback(t *testing.T) {
	var callbackPosts atomic.Int32
	cbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackPosts.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":0}`))
	}))
	t.Cleanup(cbSrv.Close)

	assetDir := t.TempDir()
	srv, err := office.New(nopStorage{}, office.Options{
		AssetDir:     assetDir,
		PublicOrigin: "http://localhost:8080",
	})
	if err != nil {
		t.Fatal(err)
	}
	docKey := "noop-force"
	srv.Sessions().UpsertDoc(session.Document{
		Key:         docKey,
		Path:        "sample-files/sample.xlsx",
		FileType:    "xlsx",
		CallbackURL: cbSrv.URL,
	})

	if err := srv.FlushDocument(context.Background(), docKey, "", true); err != nil {
		t.Fatalf("noop force flush: %v", err)
	}
	if callbackPosts.Load() != 0 {
		t.Fatalf("expected no callback POST for noop force flush, got %d", callbackPosts.Load())
	}
}

func TestCacheFileURLFallsBackToPublicOrigin(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{
		AssetDir:     t.TempDir(),
		PublicOrigin: "http://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := srv.CacheFileURL("", "doc-key", "saved.xlsx")
	want := "http://example.com/cache/files/doc-key/saved.xlsx"
	if got != want {
		t.Fatalf("CacheFileURL() = %q, want %q", got, want)
	}
}

func TestFlushDocumentUnknownKey(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{AssetDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	srv.Handler()

	err = srv.FlushDocument(context.Background(), "missing-key", "http://localhost", false)
	if err == nil {
		t.Fatal("expected error for unknown document key")
	}
}

func TestPersistDocumentNoOpWithoutEditorBin(t *testing.T) {
	assetDir := t.TempDir()
	srv, err := office.New(nopStorage{}, office.Options{AssetDir: assetDir})
	if err != nil {
		t.Fatal(err)
	}
	docKey := "test-key"
	srv.Sessions().UpsertDoc(session.Document{
		Key:      docKey,
		Path:     "sample-files/sample.csv",
		FileType: "csv",
	})

	err = srv.PersistDocument(context.Background(), docKey)
	if err != nil {
		t.Fatalf("expected no-op when Editor.bin is missing and no changes: %v", err)
	}
}

func TestPersistDocumentSkipsStaleEditorBinWithoutChanges(t *testing.T) {
	assetDir := t.TempDir()
	cacheRoot := filepath.Join(assetDir, "cache")
	docKey := "persist-key"
	cacheDir := filepath.Join(cacheRoot, docKey)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "Editor.bin"), []byte("stale-editor-bin"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &recordingStorage{t: t}
	srv, err := office.New(store, office.Options{AssetDir: assetDir})
	if err != nil {
		t.Fatal(err)
	}
	srv.Sessions().UpsertDoc(session.Document{
		Key:      docKey,
		Path:     "sample-files/sample.csv",
		FileType: "csv",
	})

	err = srv.PersistDocument(context.Background(), docKey)
	if err != nil {
		t.Fatalf("expected no-op without pending changes: %v", err)
	}
	if store.savedPath != "" {
		t.Fatalf("should not rewrite storage from stale Editor.bin, got %q", store.savedPath)
	}
}

type recordingStorage struct {
	t         *testing.T
	savedPath string
	savedBody *bytes.Buffer
}

func (s *recordingStorage) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (s *recordingStorage) Save(_ context.Context, path string, r io.Reader) error {
	s.savedPath = path
	if s.savedBody != nil {
		_, err := io.Copy(s.savedBody, r)
		return err
	}
	return nil
}

func (s *recordingStorage) Stat(context.Context, string) (office.FileInfo, error) {
	return office.FileInfo{}, os.ErrNotExist
}
