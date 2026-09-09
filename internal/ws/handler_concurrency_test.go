package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/ws"
)

type hookOpener struct {
	mu       sync.Mutex
	calls    map[string]int
	delay    time.Duration
	blockKey string
	block    chan struct{}
}

func newHookOpener() *hookOpener {
	return &hookOpener{calls: make(map[string]int)}
}

func (o *hookOpener) Open(_ context.Context, _, _, docKey string, _ ws.OpenCmd) ([]string, error) {
	if o.blockKey == docKey && o.block != nil {
		<-o.block
	}
	if o.delay > 0 {
		time.Sleep(o.delay)
	}
	o.mu.Lock()
	o.calls[docKey]++
	o.mu.Unlock()
	pkt, err := ws.DocumentOpenOKPacket("open", map[string]string{"Editor.bin": "http://test/" + docKey})
	if err != nil {
		return nil, err
	}
	return []string{pkt}, nil
}

func (o *hookOpener) count(docKey string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.calls[docKey]
}

type blockingSaver struct {
	mu       sync.Mutex
	calls    []string
	block    chan struct{}
	release  chan struct{}
	inFlight bool
}

func (s *blockingSaver) FlushDocument(_ context.Context, docKey, _ string, _ bool) error {
	s.mu.Lock()
	s.calls = append(s.calls, docKey)
	s.inFlight = true
	s.mu.Unlock()
	if s.block != nil {
		select {
		case <-s.block:
		case <-s.release:
		}
	}
	s.mu.Lock()
	s.inFlight = false
	s.mu.Unlock()
	return nil
}

func (s *blockingSaver) flushedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

func authPostBody(format, url string) string {
	return `42["message",{"type":"auth","docid":"` + format + `","user":{"id":"demo","username":"Demo"},"openCmd":{"c":"open","id":"` + format + `","format":"` + format + `","url":"` + url + `"}}]`
}

func authPostRequest(format, docURL string) *http.Request {
	return authPostRequestWithSID("go-office", format, docURL)
}

func authPostRequestWithSID(sid, format, docURL string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid="+sid,
		strings.NewReader(authPostBody(format, docURL)))
}

func handshakeSID(t *testing.T, h *ws.Handler, docKey string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling", nil)
	rec := httptest.NewRecorder()
	h.ServePath(rec, req, "/doc/"+docKey+"/c")
	body := rec.Body.String()
	const prefix = `0{"sid":"`
	if !strings.HasPrefix(body, prefix) {
		t.Fatalf("handshake body = %q", body)
	}
	rest := strings.TrimPrefix(body, prefix)
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		t.Fatalf("handshake sid missing in %q", body)
	}
	return rest[:end]
}

func pollingGet(t *testing.T, h *ws.Handler, docKey, sid string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid="+sid+"&t=1", nil)
	rec := httptest.NewRecorder()
	h.ServePath(rec, req, "/doc/"+docKey+"/c")
	return rec.Body.String()
}

func TestHandlerStopDrainsInFlightOpens(t *testing.T) {
	ws.ResetSessionsForTest()
	block := make(chan struct{})
	opener := newHookOpener()
	opener.blockKey = "blocked"
	opener.block = block
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0

	h.ServePath(httptest.NewRecorder(), authPostRequest("docx", "http://localhost/sample.docx"), "/doc/blocked/c")

	stopped := make(chan struct{})
	go func() {
		h.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned while document open still in flight")
	case <-time.After(100 * time.Millisecond):
	}

	close(block)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after open completed")
	}
}

func TestCrossDocCSVSaveDoesNotBlockDocxOpen(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	block := make(chan struct{})
	saver := &blockingSaver{block: block}
	delay := 15 * time.Millisecond
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:   "9.3.4",
		CacheDir:  t.TempDir(),
		Saver:     saver,
		SaveDelay: &delay,
		OpenHook:  opener,
	})
	h.PollHold = 0

	csvKey := "0c799d3dbda398a50f7077f6f3c3de7cb9110610d27e9318951de50ec9788e47"
	docxKey := "2988586ab196cef36a5f56792e767183c94e8215c7bf6f773604776cdb52ac25"

	csvAuth := authPostRequest("csv", "http://localhost/sample.csv")
	h.ServePath(httptest.NewRecorder(), csvAuth, "/doc/"+csvKey+"/c")

	saveBody := `42["message",{"type":"saveChanges","changes":["x"],"deleteIndex":-1,"reSave":true}]`
	saveReq := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(saveBody))
	h.ServePath(httptest.NewRecorder(), saveReq, "/doc/"+csvKey+"/c")

	time.Sleep(20 * time.Millisecond)

	docxAuth := authPostRequest("docx", "http://localhost/sample.docx")
	h.ServePath(httptest.NewRecorder(), docxAuth, "/doc/"+docxKey+"/c")

	docxDone := make(chan struct{})
	go func() {
		defer close(docxDone)
		waitDocumentOpen(t, h, docxKey, "go-office")
	}()

	select {
	case <-docxDone:
	case <-time.After(2 * time.Second):
		t.Fatal("docx open blocked while csv flush in flight")
	}

	close(block)
	time.Sleep(30 * time.Millisecond)

	if opener.count(docxKey) == 0 {
		t.Fatal("docx open was never started")
	}
	keys := saver.flushedKeys()
	if len(keys) == 0 || keys[len(keys)-1] != csvKey {
		t.Fatalf("expected csv flush, got %+v", keys)
	}
}

func TestHandlerSwitchCSVThenDocxSameSid(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0

	csvKey := "csv-switch"
	docxKey := "docx-switch"

	h.ServePath(httptest.NewRecorder(), authPostRequest("csv", "http://localhost/sample.csv"), "/doc/"+csvKey+"/c")
	waitDocumentOpen(t, h, csvKey, "go-office")

	h.ServePath(httptest.NewRecorder(), authPostRequest("docx", "http://localhost/sample.docx"), "/doc/"+docxKey+"/c")
	waitDocumentOpen(t, h, docxKey, "go-office")

	if opener.count(csvKey) == 0 || opener.count(docxKey) == 0 {
		t.Fatalf("expected both documents to open, csv=%d docx=%d", opener.count(csvKey), opener.count(docxKey))
	}
}

func reloadConnectPacket(docKey, format, docURL string) string {
	return `40{"data":{"type":"auth","docid":"` + docKey + `","user":{"id":"demo","username":"Demo"},"openCmd":{"c":"open","id":"` + docKey + `","format":"` + format + `","url":"` + docURL + `"}}}`
}

func TestHandlerIndependentTransportOutboxIsolation(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0

	csvKey := "tab-csv"
	docxKey := "tab-docx"
	csvURL := "http://localhost/sample.csv"
	docxURL := "http://localhost/sample.docx"

	sidCSV := handshakeSID(t, h, csvKey)
	sidDocx := handshakeSID(t, h, docxKey)

	h.ServePath(httptest.NewRecorder(), authPostRequestWithSID(sidCSV, "csv", csvURL), "/doc/"+csvKey+"/c")
	h.ServePath(httptest.NewRecorder(), authPostRequestWithSID(sidDocx, "docx", docxURL), "/doc/"+docxKey+"/c")
	waitDocumentOpen(t, h, csvKey, sidCSV)
	waitDocumentOpen(t, h, docxKey, sidDocx)

	csvPoll := pollingGet(t, h, csvKey, sidCSV)
	docxPoll := pollingGet(t, h, docxKey, sidDocx)
	if strings.Contains(csvPoll, docxKey) {
		t.Fatalf("csv transport outbox leaked docx key: %s", csvPoll)
	}
	if strings.Contains(docxPoll, csvKey) {
		t.Fatalf("docx transport outbox leaked csv key: %s", docxPoll)
	}
	if sidCSV == sidDocx {
		t.Fatalf("expected distinct transport sids, both %q", sidCSV)
	}
}

func TestHandlerReloadWithFreshTransportOpens(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0
	docKey := "reload-open-key"
	docURL := "http://localhost/a.docx"

	sid1 := handshakeSID(t, h, docKey)
	h.ServePath(httptest.NewRecorder(), authPostRequestWithSID(sid1, "docx", docURL), "/doc/"+docKey+"/c")
	waitDocumentOpen(t, h, docKey, sid1)

	sid2 := handshakeSID(t, h, docKey)
	reload := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid="+sid2,
		strings.NewReader(reloadConnectPacket(docKey, "docx", docURL)))
	h.ServePath(httptest.NewRecorder(), reload, "/doc/"+docKey+"/c")
	waitDocumentOpen(t, h, docKey, sid2)

	if opener.count(docKey) < 2 {
		t.Fatalf("open calls = %d, want 2 after fresh transport reload", opener.count(docKey))
	}
}

func TestHandlerReloadAfterSessionReset(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0
	docKey := "reload-ok-key"
	docURL := "http://localhost/a.docx"

	h.ServePath(httptest.NewRecorder(), authPostRequest("docx", docURL), "/doc/"+docKey+"/c")
	waitDocumentOpen(t, h, docKey, "go-office")

	ws.ClearDocumentSession(docKey)
	reload := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office",
		strings.NewReader(reloadConnectPacket(docKey, "docx", docURL)))
	h.ServePath(httptest.NewRecorder(), reload, "/doc/"+docKey+"/c")
	waitDocumentOpen(t, h, docKey, "go-office")

	if opener.count(docKey) < 2 {
		t.Fatalf("expected 2 opens after session reset reload, got %d", opener.count(docKey))
	}
}

func TestHandlerReopenAfterFirstOpenCompletes(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0
	docKey := "same-key"
	docURL := "http://localhost/a.docx"

	h.ServePath(httptest.NewRecorder(), authPostRequest("docx", docURL), "/doc/"+docKey+"/c")
	waitDocumentOpen(t, h, docKey, "go-office")

	ws.ClearDocumentSession(docKey)
	reloadConnect := `40{"data":{"type":"auth","docid":"` + docKey + `","user":{"id":"demo","username":"Demo"},"openCmd":{"c":"open","id":"` + docKey + `","format":"docx","url":"` + docURL + `"}}}`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(reloadConnect)), "/doc/"+docKey+"/c")
	waitDocumentOpen(t, h, docKey, "go-office")

	if opener.count(docKey) < 2 {
		t.Fatalf("expected 2 opens on reload, got %d", opener.count(docKey))
	}
}

func waitDocumentOpen(t *testing.T, h *ws.Handler, docKey, sid string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		body := pollingGet(t, h, docKey, sid)
		if strings.Contains(body, `"type":"documentOpen"`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("documentOpen not received for %s", docKey)
}
