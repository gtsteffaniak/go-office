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
