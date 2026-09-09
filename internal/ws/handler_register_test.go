package ws

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type mockSessionRegistrar struct {
	mu   sync.Mutex
	calls []struct {
		key, callback, fileType, docURL string
	}
}

func (m *mockSessionRegistrar) FlushDocument(context.Context, string, string, bool) error {
	return nil
}

func (m *mockSessionRegistrar) RegisterDocumentSession(docKey, callbackURL, fileType, documentURL, userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, struct {
		key, callback, fileType, docURL string
	}{docKey, callbackURL, fileType, documentURL})
	_ = userID
}

func TestHandlerRegisterDocumentSessionOnAuth(t *testing.T) {
	ResetSessionsForTest()
	reg := &mockSessionRegistrar{}
	h := NewWithOptions(HandlerOptions{
		Version:  "9.3.4-hotfix.1",
		CacheDir: t.TempDir(),
		Saver:    reg,
	})
	h.PollHold = 0

	docKey := "integrator-key"
	authBody := `42["message",{"type":"auth","docid":"` + docKey + `","documentCallbackUrl":"http://host/callback","user":{"id":"u1"},"openCmd":{"c":"open","id":"` + docKey + `","format":"xlsx","url":"http://host/view"}}]`
	req := httptest.NewRequest("POST", "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(authBody))
	h.ServePath(httptest.NewRecorder(), req, "/doc/"+docKey+"/c")

	reg.mu.Lock()
	defer reg.mu.Unlock()
	if len(reg.calls) != 1 {
		t.Fatalf("RegisterDocumentSession calls = %d, want 1", len(reg.calls))
	}
	call := reg.calls[0]
	if call.key != docKey || call.callback != "http://host/callback" || call.fileType != "xlsx" || call.docURL != "http://host/view" {
		t.Fatalf("unexpected registration: %+v", call)
	}
}
