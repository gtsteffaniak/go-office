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
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
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
	csvBody := []byte("a,b\n1,2\n")
	tmp := filepath.Join(cacheDir, "src.csv")
	if err := os.WriteFile(tmp, csvBody, 0o644); err != nil {
		t.Fatal(err)
	}
	srcHash, err := convert.FileSHA256(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "Editor.bin"), []byte("editor-bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := convert.WriteSourceHash(outDir, srcHash); err != nil {
		t.Fatal(err)
	}

	downloads := 0
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		_, _ = w.Write(csvBody)
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
	if downloads != 1 {
		t.Fatalf("download calls = %d, want 1 hash check on fresh cache hit", downloads)
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

func TestCoauthoringOriginPrefersPublicOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/demo", nil)
	req.Host = "localhost:9999"
	if got := CoauthoringOrigin("https://docs.example.com", req); got != "https://docs.example.com" {
		t.Fatalf("public origin = %q, want https://docs.example.com", got)
	}
	if got := CoauthoringOrigin("", req); got != "http://localhost:9999" {
		t.Fatalf("request origin = %q, want http://localhost:9999", got)
	}
}

func TestOpenerOpenFastPathWithPendingChangesAndValidCache(t *testing.T) {
	cacheDir := t.TempDir()
	key := "fast-open"
	outDir := filepath.Join(cacheDir, key)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	csvBody := []byte("a,b\n1,2\n")
	tmp := filepath.Join(cacheDir, "src.csv")
	if err := os.WriteFile(tmp, csvBody, 0o644); err != nil {
		t.Fatal(err)
	}
	srcHash, err := convert.FileSHA256(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "Editor.bin"), []byte("editor-bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := convert.WriteSourceHash(outDir, srcHash); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(outDir, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["chg"]`), 0o644); err != nil {
		t.Fatal(err)
	}

	block := make(chan struct{})
	saver := &blockingFlushSaver{block: block}
	opener := &Opener{CacheDir: cacheDir, Saver: saver, Logger: slog.Default()}

	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(csvBody)
	}))
	t.Cleanup(fileSrv.Close)

	start := time.Now()
	packets, err := opener.Open(context.Background(), "http://example.com", "/office", key, openCmd{
		Command: "open",
		Format:  "csv",
		URL:     fileSrv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("open blocked on flush for %v", time.Since(start))
	}
	if len(packets) != 1 || !strings.Contains(packets[0], `"status":"ok"`) {
		t.Fatalf("unexpected packet: %v", packets)
	}

	close(block)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if saver.calls >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected background flush for orphaned pending changes")
}

type blockingFlushSaver struct {
	block chan struct{}
	calls int
}

func (s *blockingFlushSaver) FlushDocument(context.Context, string, string, bool) error {
	s.calls++
	if s.block != nil {
		<-s.block
	}
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
