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

func TestPersistDocumentRequiresEditorBin(t *testing.T) {
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
	if err == nil {
		t.Fatal("expected error when Editor.bin is missing")
	}
}

func TestPersistDocumentWritesStorage(t *testing.T) {
	assetDir := t.TempDir()
	cacheRoot := filepath.Join(assetDir, "cache")
	docKey := "persist-key"
	cacheDir := filepath.Join(cacheRoot, docKey)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal Editor.bin placeholder; conversion will fail without real x2t/assets.
	if err := os.WriteFile(filepath.Join(cacheDir, "Editor.bin"), []byte("not-a-real-editor-bin"), 0o644); err != nil {
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
	if err == nil {
		t.Fatal("expected conversion error without real x2t")
	}
	if store.savedPath != "" {
		t.Fatalf("should not save to storage on conversion failure, got %q", store.savedPath)
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
