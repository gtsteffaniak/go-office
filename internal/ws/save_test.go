package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/ws"
)

type nopSaver struct{}

func (nopSaver) FlushDocument(context.Context, string, string, bool) error { return nil }

func saveTestHandler(t *testing.T) *ws.Handler {
	t.Helper()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4-hotfix.1",
		CacheDir: t.TempDir(),
		Saver:    nopSaver{},
	})
	h.PollHold = 0
	return h
}

func TestPollingSaveLock(t *testing.T) {
	h := saveTestHandler(t)

	lockPost := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"isSaveLock"}]`))
	h.ServePath(httptest.NewRecorder(), lockPost, "/doc/key/c")

	get := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=1", nil)
	rec := httptest.NewRecorder()
	h.ServePath(rec, get, "/doc/key/c")
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"saveLock"`) {
		t.Fatalf("saveLock missing: %q", body)
	}
}

func TestPollingSaveChangesUnlock(t *testing.T) {
	h := saveTestHandler(t)

	savePost := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"saveChanges","changes":["chg1"],"deleteIndex":-1}]`))
	h.ServePath(httptest.NewRecorder(), savePost, "/doc/key/c")

	get := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=2", nil)
	rec := httptest.NewRecorder()
	h.ServePath(rec, get, "/doc/key/c")
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"unSaveLock"`) {
		t.Fatalf("unSaveLock missing: %q", body)
	}
}
