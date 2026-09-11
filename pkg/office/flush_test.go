package office_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/session"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

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
