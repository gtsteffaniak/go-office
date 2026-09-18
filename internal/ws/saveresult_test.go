package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/ws"
)

// detailedSaver records flushes and returns FlushDetail, exercising ws.DetailedDocumentSaver.
type detailedSaver struct {
	calls  []flushCall
	detail ws.FlushDetail
	err    error
}

func (s *detailedSaver) FlushDocument(_ context.Context, docKey, origin string, force bool) error {
	_, err := s.FlushDocumentDetailed(context.Background(), docKey, origin, force)
	return err
}

func (s *detailedSaver) FlushDocumentDetailed(_ context.Context, docKey, origin string, force bool) (ws.FlushDetail, error) {
	s.calls = append(s.calls, flushCall{docKey: docKey, origin: origin, force: force})
	return s.detail, s.err
}

// TestSaveOutcomeBroadcastOnSuccess verifies the server reports a successful flush so
// clients do not have to infer saves by polling storage.
func TestSaveOutcomeBroadcastOnSuccess(t *testing.T) {
	ws.ResetSessionsForTest()
	ws.ResetSaveOutcomesForTest()
	saver := &detailedSaver{detail: ws.FlushDetail{RolledBack: true, Bridge: "xlsx", Bytes: 4242}}
	delay := 10 * time.Millisecond
	h := saveTestHandlerWithSaver(t, saver, delay)

	body := `42["message",{"type":"saveChanges","changes":["c1"],"reIndex":1,"deleteIndex":-1,"reSave":true}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body)), "/doc/key/c")

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := ws.LastSaveOutcome("key"); ok {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	outcome, ok := ws.LastSaveOutcome("key")
	if !ok {
		t.Fatal("expected a recorded save outcome after a successful flush")
	}
	if !outcome.Success {
		t.Fatalf("outcome should report success: %+v", outcome)
	}
	if !outcome.RolledBack || outcome.Bridge != "xlsx" {
		t.Fatalf("outcome should carry rollback detail: %+v", outcome)
	}
	if outcome.Bytes != 4242 {
		t.Fatalf("outcome bytes = %d, want 4242", outcome.Bytes)
	}
	if outcome.Sequence == 0 {
		t.Fatal("outcome should carry a non-zero sequence")
	}

	// The event must also appear on the coauthoring transport for the editor client.
	rec := httptest.NewRecorder()
	h.ServePath(rec, httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=sr", nil), "/doc/key/c")
	if out := rec.Body.String(); !strings.Contains(out, `"type":"saveResult"`) {
		t.Fatalf("expected a saveResult packet on the coauthoring transport, got %q", out)
	}
}

// TestSaveOutcomeBroadcastOnFailure is the fail-fast guarantee: a failed flush must be
// reported to the client. sdkjs clears its 60s save-retry timer on unSaveLock, which the
// server sends before the flush runs, so without this event a failed save is invisible and
// the only symptom is a client waiting out a long timeout.
func TestSaveOutcomeBroadcastOnFailure(t *testing.T) {
	ws.ResetSessionsForTest()
	ws.ResetSaveOutcomesForTest()
	saver := &recordingSaver{err: errFlush}
	delay := 10 * time.Millisecond
	h := saveTestHandlerWithSaver(t, saver, delay)

	body := `42["message",{"type":"saveChanges","changes":["c1"],"deleteIndex":-1,"reSave":true}]`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body)), "/doc/key/c")

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := ws.LastSaveOutcome("key"); ok {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	outcome, ok := ws.LastSaveOutcome("key")
	if !ok {
		t.Fatal("expected a recorded save outcome after a failed flush")
	}
	if outcome.Success {
		t.Fatal("outcome should report failure")
	}
	if outcome.Error == "" {
		t.Fatal("failed outcome should carry the error reason")
	}
}

// TestSaveOutcomeSequenceIncreases ensures clients can order events and detect staleness.
func TestSaveOutcomeSequenceIncreases(t *testing.T) {
	ws.ResetSessionsForTest()
	ws.ResetSaveOutcomesForTest()
	saver := &detailedSaver{}
	delay := 5 * time.Millisecond
	h := saveTestHandlerWithSaver(t, saver, delay)

	body := `42["message",{"type":"saveChanges","changes":["c1"],"deleteIndex":-1,"reSave":true}]`
	for i := 0; i < 3; i++ {
		h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body)), "/doc/key/c")
		time.Sleep(30 * time.Millisecond)
	}

	outcome, ok := ws.LastSaveOutcome("key")
	if !ok {
		t.Fatal("expected a recorded save outcome")
	}
	if outcome.Sequence < 2 {
		t.Fatalf("sequence should advance across flushes, got %d", outcome.Sequence)
	}
}
