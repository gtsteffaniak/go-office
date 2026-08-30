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
