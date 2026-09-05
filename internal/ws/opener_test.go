package ws

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenerOpenPDF(t *testing.T) {
	pdf := []byte("%PDF-1.4\n%EOF\n")
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdf)
	}))
	t.Cleanup(fileSrv.Close)

	cacheDir := t.TempDir()
	opener := &Opener{
		CacheDir: cacheDir,
		Logger:   slog.Default(),
	}
	packets, err := opener.Open(context.Background(), "http://example.com", "/office", "doc-key", openCmd{
		Command: "open",
		Format:  "pdf",
		URL:     fileSrv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 1 {
		t.Fatalf("packets = %d, want 1", len(packets))
	}
	if !strings.Contains(packets[0], `"status":"ok"`) {
		t.Fatalf("packet status not ok: %s", packets[0])
	}
	if !strings.Contains(packets[0], `"origin.pdf"`) {
		t.Fatalf("packet missing origin.pdf: %s", packets[0])
	}
	if !strings.Contains(packets[0], `/office/cache/files/doc-key/origin.pdf`) {
		t.Fatalf("packet missing cache url: %s", packets[0])
	}
	data, err := os.ReadFile(filepath.Join(cacheDir, "doc-key", "origin.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(pdf) {
		t.Fatalf("cached pdf = %q, want %q", data, pdf)
	}
}

func TestOpenerOpenSkipsDownloadWhenEditorBinCached(t *testing.T) {
	cacheDir := t.TempDir()
	key := "cached-key"
	outDir := filepath.Join(cacheDir, key)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "Editor.bin"), []byte("editor-bin"), 0o644); err != nil {
		t.Fatal(err)
	}

	downloads := 0
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		http.Error(w, "should not download", http.StatusInternalServerError)
	}))
	t.Cleanup(fileSrv.Close)

	opener := &Opener{CacheDir: cacheDir, Logger: slog.Default()}
	packets, err := opener.Open(context.Background(), "http://example.com", "/office", key, openCmd{
		Command: "open",
		Format:  "csv",
		URL:     fileSrv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if downloads != 0 {
		t.Fatalf("download calls = %d, want 0 on cache hit", downloads)
	}
	if len(packets) != 1 || !strings.Contains(packets[0], `"status":"ok"`) {
		t.Fatalf("unexpected packet: %v", packets)
	}
	if !strings.Contains(packets[0], `"Editor.bin"`) {
		t.Fatalf("packet missing Editor.bin: %s", packets[0])
	}
}

type recordingFlushSaver struct {
	calls int
}

func (s *recordingFlushSaver) FlushDocument(context.Context, string, string, bool) error {
	s.calls++
	return nil
}

func TestOpenerFlushPendingBeforeOpen(t *testing.T) {
	cacheDir := t.TempDir()
	key := "csv-key"
	changesDir := filepath.Join(cacheDir, key, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["chg"]`), 0o644); err != nil {
		t.Fatal(err)
	}

	saver := &recordingFlushSaver{}
	opener := &Opener{CacheDir: cacheDir, Saver: saver}
	if err := opener.flushPending(context.Background(), key, "http://localhost"); err != nil {
		t.Fatal(err)
	}
	if saver.calls != 1 {
		t.Fatalf("flush calls = %d, want 1 for pending changes", saver.calls)
	}

	if err := opener.flushPending(context.Background(), "other-key", "http://localhost"); err != nil {
		t.Fatal(err)
	}
	if saver.calls != 1 {
		t.Fatalf("no pending changes should not flush again, calls=%d", saver.calls)
	}
}
