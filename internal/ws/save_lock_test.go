package ws_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/ws"
)

func TestIsSaveLockBlocksWhileFlushRunning(t *testing.T) {
	ws.ResetSessionsForTest()
	block := make(chan struct{})
	started := make(chan struct{}, 1)
	saver := &recordingSaver{block: block, started: started}
	delay := time.Hour
	h := saveTestHandlerWithSaver(t, saver, delay)

	go func() {
		body := `42["message",{"type":"saveChanges","changes":["c1"],"reSave":true,"deleteIndex":-1}]`
		req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
		h.ServePath(httptest.NewRecorder(), req, "/doc/key/c")
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("flush did not start")
	}

	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"isSaveLock"}]`)), "/doc/key/c")
	rec := httptest.NewRecorder()
	h.ServePath(rec, httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=lock", nil), "/doc/key/c")
	out := rec.Body.String()
	if !strings.Contains(out, `"saveLock":true`) {
		t.Fatalf("expected saveLock while flush running, got %q", out)
	}

	close(block)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
			strings.NewReader(`42["message",{"type":"isSaveLock"}]`)), "/doc/key/c")
		rec = httptest.NewRecorder()
		h.ServePath(rec, httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=unlock", nil), "/doc/key/c")
		if strings.Contains(rec.Body.String(), `"saveLock":false`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected saveLock to clear after flush, last=%q", rec.Body.String())
}
