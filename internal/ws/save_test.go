package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestPollingExcelStringChangesWritten(t *testing.T) {
	ws.ResetSessionsForTest()
	cacheDir := t.TempDir()
	delay := time.Hour
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:   "9.3.4-hotfix.1",
		CacheDir:  cacheDir,
		Saver:     nopSaver{},
		SaveDelay: &delay,
	})
	h.PollHold = 0

	body := `42["message",{"type":"saveChanges","changes":"[\"14;CgAAAAFiAAAA/wAAAAA=\",\"128;fAAAAAFkBwAABXIAAAAAAAE=\"]","startSaveChanges":true,"endSaveChanges":true,"isExcel":true,"deleteIndex":null}]`
	req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
	h.ServePath(httptest.NewRecorder(), req, "/doc/csv-key/c")

	raw, err := os.ReadFile(filepath.Join(cacheDir, "csv-key", "changes", "changes0.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "14;CgAAAAFiAAAA/wAAAAA=") {
		t.Fatalf("excel change blobs missing from changes0.json: %s", raw)
	}
}

func TestPollingGetLockExcel(t *testing.T) {
	ws.ResetSessionsForTest()
	h := saveTestHandler(t)

	body := `42["message",{"type":"getLock","block":[{"sheetId":"5","type":1,"guid":"lock-guid-a2","rangeOrObjectId":{"c1":0,"r1":1,"c2":0,"r2":1}}]}]`
	req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
	h.ServePath(httptest.NewRecorder(), req, "/doc/csv-key/c")

	get := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=1", nil)
	rec := httptest.NewRecorder()
	h.ServePath(rec, get, "/doc/csv-key/c")
	out := rec.Body.String()
	for _, want := range []string{`"type":"getLock"`, `"lock-guid-a2"`, `"user":"user1"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("getLock reply missing %s: %q", want, out)
		}
	}
}

func TestPollingGetLockUsesAuthUser(t *testing.T) {
	ws.ResetSessionsForTest()
	h := saveTestHandler(t)

	auth := `42["message",{"type":"auth","docid":"csv-key","user":{"id":"demo-user","username":"Demo"}}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(auth)), "/doc/csv-key/c")
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=auth", nil), "/doc/csv-key/c")

	lock := `42["message",{"type":"getLock","block":[{"guid":"cell-1","sheetId":"5","type":1}]}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(lock)), "/doc/csv-key/c")

	rec := httptest.NewRecorder()
	h.ServePath(rec, httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=lock", nil), "/doc/csv-key/c")
	out := rec.Body.String()
	if !strings.Contains(out, `"type":"getLock"`) || !strings.Contains(out, `"user":"demo-user1"`) {
		t.Fatalf("lock user should match sdkjs id+indexUser: %q", out)
	}
}

func TestPollingForceSaveStartNotModified(t *testing.T) {
	ws.ResetSessionsForTest()
	fallback := 100 * time.Millisecond
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:                "9.3.4-hotfix.1",
		CacheDir:               t.TempDir(),
		Saver:                  nopSaver{},
		ForceSaveFallbackDelay: &fallback,
	})
	h.PollHold = 0

	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"forceSaveStart"}]`)), "/doc/csv-key/c")

	time.Sleep(200 * time.Millisecond)

	rec := httptest.NewRecorder()
	h.ServePath(rec, httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=fs", nil), "/doc/csv-key/c")
	out := rec.Body.String()
	if !strings.Contains(out, `"type":"forceSave"`) || !strings.Contains(out, `"success":false`) {
		t.Fatalf("empty force save should report failure: %q", out)
	}
}

func TestPollingForceSaveStartFlushesPendingChanges(t *testing.T) {
	ws.ResetSessionsForTest()
	saver := &recordingSaver{}
	delay := time.Hour
	h := saveTestHandlerWithSaver(t, saver, delay)

	changes := `42["message",{"type":"saveChanges","changes":["cell-edit"],"isExcel":true,"deleteIndex":null}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(changes)), "/doc/csv-key/c")
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=unsave", nil), "/doc/csv-key/c")

	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"forceSaveStart"}]`)), "/doc/csv-key/c")

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(saver.flushCalls()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	calls := saver.flushCalls()
	if len(calls) != 1 || !calls[0].force {
		t.Fatalf("forceSaveStart should force flush: %+v", calls)
	}

	rec := httptest.NewRecorder()
	h.ServePath(rec, httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=fs", nil), "/doc/csv-key/c")
	out := rec.Body.String()
	if !strings.Contains(out, `"type":"forceSaveStart"`) || !strings.Contains(out, `"code":0`) {
		t.Fatalf("forceSaveStart ack missing: %q", out)
	}
	if !strings.Contains(out, `"type":"forceSave"`) || !strings.Contains(out, `"success":true`) {
		t.Fatalf("forceSave success missing: %q", out)
	}
}

func TestPollingForceSaveStartWaitsForEndSaveChanges(t *testing.T) {
	ws.ResetSessionsForTest()
	saver := &recordingSaver{}
	delay := time.Hour
	h := saveTestHandlerWithSaver(t, saver, delay)

	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"forceSaveStart"}]`)), "/doc/csv-key/c")

	time.Sleep(50 * time.Millisecond)
	if len(saver.flushCalls()) != 0 {
		t.Fatalf("forceSaveStart should wait for saveChanges before flush: %+v", saver.flushCalls())
	}

	changes := `42["message",{"type":"saveChanges","changes":["cell-edit"],"isExcel":true,"startSaveChanges":true,"endSaveChanges":true,"deleteIndex":null}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(changes)), "/doc/csv-key/c")

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(saver.flushCalls()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	calls := saver.flushCalls()
	if len(calls) != 1 || !calls[0].force {
		t.Fatalf("endSaveChanges should complete force save: %+v", calls)
	}
}

func TestPollingForceSaveStartLateEndSaveChanges(t *testing.T) {
	ws.ResetSessionsForTest()
	saver := &recordingSaver{}
	delay := time.Hour
	fallback := 2 * time.Second
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:                "9.3.4-hotfix.1",
		CacheDir:               t.TempDir(),
		Saver:                  saver,
		SaveDelay:              &delay,
		ForceSaveFallbackDelay: &fallback,
	})
	h.PollHold = 0

	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"forceSaveStart"}]`)), "/doc/csv-key/c")

	time.Sleep(400 * time.Millisecond)
	partial := `42["message",{"type":"saveChanges","changes":["cell-edit"],"isExcel":true,"deleteIndex":null}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(partial)), "/doc/csv-key/c")

	if len(saver.flushCalls()) != 0 {
		t.Fatalf("partial saveChanges should not flush before endSaveChanges: %+v", saver.flushCalls())
	}

	time.Sleep(200 * time.Millisecond)
	end := `42["message",{"type":"saveChanges","changes":["cell-edit-2"],"isExcel":true,"startSaveChanges":true,"endSaveChanges":true,"deleteIndex":null}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(end)), "/doc/csv-key/c")

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(saver.flushCalls()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	calls := saver.flushCalls()
	if len(calls) != 1 || !calls[0].force {
		t.Fatalf("late endSaveChanges should complete force save once: %+v", calls)
	}
}

func TestPollingEndSaveChangesBeforeForceSaveStart(t *testing.T) {
	ws.ResetSessionsForTest()
	saver := &recordingSaver{}
	delay := time.Hour
	h := saveTestHandlerWithSaver(t, saver, delay)

	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"isSaveLock"}]`)), "/doc/xlsm-key/c")

	end := `42["message",{"type":"saveChanges","changes":["cell-edit"],"isExcel":true,"startSaveChanges":true,"endSaveChanges":true,"deleteIndex":null}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(end)), "/doc/xlsm-key/c")

	time.Sleep(20 * time.Millisecond)
	if len(saver.flushCalls()) != 0 {
		t.Fatalf("endSaveChanges after isSaveLock should wait for forceSaveStart: %+v", saver.flushCalls())
	}

	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(`42["message",{"type":"forceSaveStart"}]`)), "/doc/xlsm-key/c")

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(saver.flushCalls()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	calls := saver.flushCalls()
	if len(calls) != 1 {
		t.Fatalf("expected single immediate flush after forceSaveStart, got %+v", calls)
	}
	if !calls[0].force {
		t.Fatalf("save button flush should be force=true: %+v", calls)
	}
}
