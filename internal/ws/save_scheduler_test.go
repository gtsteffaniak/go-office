package ws_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/ws"
)

type recordingSaver struct {
	mu    sync.Mutex
	calls []flushCall
	err   error
	block chan struct{}
}

type flushCall struct {
	docKey string
	origin string
	force  bool
}

func (s *recordingSaver) FlushDocument(_ context.Context, docKey, origin string, force bool) error {
	if s.block != nil {
		<-s.block
	}
	s.mu.Lock()
	s.calls = append(s.calls, flushCall{docKey: docKey, origin: origin, force: force})
	s.mu.Unlock()
	return s.err
}

func (s *recordingSaver) flushCalls() []flushCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]flushCall, len(s.calls))
	copy(out, s.calls)
	return out
}

func saveTestHandlerWithSaver(t *testing.T, saver ws.DocumentSaver, delay time.Duration) *ws.Handler {
	t.Helper()
	ws.ResetSessionsForTest()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:   "9.3.4-hotfix.1",
		CacheDir:  t.TempDir(),
		Saver:     saver,
		SaveDelay: &delay,
	})
	h.PollHold = 0
	return h
}

func TestSaveSchedulerDebounce(t *testing.T) {
	saver := &recordingSaver{}
	delay := 30 * time.Millisecond
	h := saveTestHandlerWithSaver(t, saver, delay)

	postSave := func() {
		body := `42["message",{"type":"saveChanges","changes":["c1"],"deleteIndex":-1}]`
		req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
		h.ServePath(httptest.NewRecorder(), req, "/doc/key1/c")
	}

	postSave()
	postSave()
	time.Sleep(delay + 40*time.Millisecond)

	calls := saver.flushCalls()
	if len(calls) != 1 {
		t.Fatalf("flush calls = %d, want 1 debounced flush: %+v", len(calls), calls)
	}
	if calls[0].docKey != "key1" {
		t.Fatalf("flush key = %q", calls[0].docKey)
	}
}

func TestSaveSchedulerForceFlush(t *testing.T) {
	saver := &recordingSaver{}
	delay := 500 * time.Millisecond
	h := saveTestHandlerWithSaver(t, saver, delay)

	body := `42["message",{"type":"saveChanges","changes":["c1"],"deleteIndex":-1,"reSave":true}]`
	req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
	h.ServePath(httptest.NewRecorder(), req, "/doc/key1/c")
	time.Sleep(20 * time.Millisecond)

	calls := saver.flushCalls()
	if len(calls) != 1 {
		t.Fatalf("force flush calls = %d, want 1", len(calls))
	}
	if !calls[0].force {
		t.Fatal("expected force flush")
	}
}

func TestSaveSchedulerPerKeyIsolation(t *testing.T) {
	saver := &recordingSaver{}
	delay := 25 * time.Millisecond
	h := saveTestHandlerWithSaver(t, saver, delay)

	for _, key := range []string{"csv-key", "docx-key"} {
		body := `42["message",{"type":"saveChanges","changes":["c"],"deleteIndex":-1}]`
		req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
		h.ServePath(httptest.NewRecorder(), req, "/doc/"+key+"/c")
	}
	time.Sleep(delay + 50*time.Millisecond)

	calls := saver.flushCalls()
	if len(calls) != 2 {
		t.Fatalf("flush calls = %d, want 2: %+v", len(calls), calls)
	}
	keys := map[string]bool{calls[0].docKey: true, calls[1].docKey: true}
	if !keys["csv-key"] || !keys["docx-key"] {
		t.Fatalf("unexpected keys: %+v", calls)
	}
}

func TestSaveSchedulerCSVStringChangesThenDocxFlush(t *testing.T) {
	saver := &recordingSaver{}
	delay := 10 * time.Millisecond
	h := saveTestHandlerWithSaver(t, saver, delay)

	csvBody := `42["message",{"type":"saveChanges","changes":"[\"14;CgAAAAFiAAAA/wAAAAA=\"]","isExcel":true,"reSave":true}]`
	csvReq := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(csvBody))
	h.ServePath(httptest.NewRecorder(), csvReq, "/doc/csv-key/c")

	docxBody := `42["message",{"type":"saveChanges","changes":["docx"],"reSave":true}]`
	docxReq := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(docxBody))
	h.ServePath(httptest.NewRecorder(), docxReq, "/doc/docx-key/c")

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(saver.flushCalls()) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	calls := saver.flushCalls()
	if len(calls) < 2 {
		t.Fatalf("flush calls = %d, want both csv and docx: %+v", len(calls), calls)
	}
	seen := map[string]bool{}
	for _, c := range calls {
		seen[c.docKey] = true
	}
	if !seen["csv-key"] || !seen["docx-key"] {
		t.Fatalf("expected independent flushes after switching documents: %+v", calls)
	}
}

func TestSaveSchedulerConcurrentSaveChanges(t *testing.T) {
	saver := &recordingSaver{}
	delay := 40 * time.Millisecond
	h := saveTestHandlerWithSaver(t, saver, delay)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			body := `42["message",{"type":"saveChanges","changes":["c"],"deleteIndex":-1}]`
			req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
			h.ServePath(httptest.NewRecorder(), req, "/doc/key/c")
		}(i)
	}
	wg.Wait()
	time.Sleep(delay + 60*time.Millisecond)

	if len(saver.flushCalls()) == 0 {
		t.Fatal("expected at least one flush after concurrent saveChanges")
	}
}

func TestSaveSchedulerFlushFailureNotifiesEditor(t *testing.T) {
	ws.ResetSessionsForTest()
	saver := &recordingSaver{err: errFlush}
	delay := 10 * time.Millisecond
	cacheDir := t.TempDir()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:   "9.3.4",
		CacheDir:  cacheDir,
		Saver:     saver,
		SaveDelay: &delay,
	})
	h.PollHold = 0

	changesDir := filepath.Join(cacheDir, "key", "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["edit"]`), 0o644); err != nil {
		t.Fatal(err)
	}

	body := `42["message",{"type":"saveChanges","changes":["edit"],"deleteIndex":-1}]`
	req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
	h.ServePath(httptest.NewRecorder(), req, "/doc/key/c")
	time.Sleep(delay + 40 * time.Millisecond)

	rec := httptest.NewRecorder()
	h.ServePath(rec, httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=fail", nil), "/doc/key/c")
	out := rec.Body.String()
	if !strings.Contains(out, `"type":"forceSave"`) || !strings.Contains(out, `"success":false`) {
		t.Fatalf("expected forceSave failure packet after flush error: %q", out)
	}
}

func TestSaveSchedulerFlushFailureKeepsChanges(t *testing.T) {
	saver := &recordingSaver{err: errFlush}
	delay := 10 * time.Millisecond
	cacheDir := t.TempDir()
	ws.ResetSessionsForTest()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:   "9.3.4",
		CacheDir:  cacheDir,
		Saver:     saver,
		SaveDelay: &delay,
	})
	h.PollHold = 0

	body := `42["message",{"type":"saveChanges","changes":["testdata"],"deleteIndex":-1,"reSave":true}]`
	req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
	h.ServePath(httptest.NewRecorder(), req, "/doc/key/c")
	time.Sleep(delay + 30*time.Millisecond)

	changesDir := filepath.Join(cacheDir, "key", "changes")
	data, err := os.ReadFile(filepath.Join(changesDir, "changes0.json"))
	if err != nil {
		t.Fatalf("changes0.json should still exist after failed flush: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected change data to remain after flush failure")
	}
}

var errFlush = errors.New("flush failed")
