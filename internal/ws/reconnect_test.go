package ws

import (
	"net/http/httptest"
	"testing"
	"time"
)

type noopTransport struct{}

func (noopTransport) writePackets([]string) error { return nil }

func TestAdoptWebSocketReplacesPrevious(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")

	first := &wsTransport{lastRead: time.Now()}
	second := &wsTransport{lastRead: time.Now()}

	sess.adoptWebSocket(first)
	sess.adoptWebSocket(second)

	sess.mu.Lock()
	active := sess.transport
	sess.mu.Unlock()
	if active != second {
		t.Fatalf("expected second websocket to be active, got %v", active)
	}
	if first.alive() {
		t.Fatal("first websocket should be closed when replaced")
	}
}

func TestShouldStartOpenSkipsPollingWhenWebSocketActive(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")
	sess.mu.Lock()
	sess.transport = noopTransport{}
	sess.docOpenDone = true
	sess.openedKey = "doc-key"
	sess.mu.Unlock()

	req := authRequest{DocID: "doc-key", Open: &openCmd{ID: "doc-key", Command: "open"}}
	if sess.shouldStartOpen(req, true) {
		t.Fatal("polling auth should not reopen while websocket is active")
	}
}

func TestShouldStartOpenAllowsReloadAfterStaleTransport(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")
	sess.mu.Lock()
	sess.transport = &wsTransport{closed: true}
	sess.docOpenDone = true
	sess.openedKey = "doc-key"
	sess.mu.Unlock()

	req := authRequest{DocID: "doc-key", Open: &openCmd{ID: "doc-key", Command: "open"}}
	if !sess.shouldStartOpen(req, true) {
		t.Fatal("polling auth should reopen when the previous websocket is gone")
	}
}

func TestOnConnectResetsStaleSessionForFreshEditor(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")
	sess.mu.Lock()
	sess.transport = noopTransport{}
	sess.docOpenDone = true
	sess.openedKey = "doc-key"
	sess.authSent = true
	sess.mu.Unlock()

	raw := connectAuthData(`40{"data":{"type":"auth","docid":"doc-key","sessionId":null,"user":{"id":"demo"},"openCmd":{"c":"open","id":"doc-key","format":"csv","url":"http://localhost/f.csv"}}}`)
	sess.onConnect(raw)

	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.docOpenDone {
		t.Fatal("fresh editor auth should reset docOpenDone")
	}
	if sess.transport != nil {
		t.Fatal("fresh editor auth should clear stale transport")
	}
}

func TestOnConnectSkipsAuthReplayWhenDocumentOpen(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")
	sess.mu.Lock()
	sess.transport = noopTransport{}
	sess.docOpenDone = true
	sess.openedKey = "doc-key"
	sess.mu.Unlock()

	raw := connectAuthData(`40{"data":{"type":"auth","docid":"doc-key","sessionId":"existing","user":{"id":"demo"},"openCmd":{"c":"open","id":"doc-key","format":"xlsx","url":"http://localhost/f.xlsx"}}}`)
	sess.onConnect(raw)
	packets := sess.drain()
	if authCount(packets) != 0 {
		t.Fatalf("expected namespace ack only on reconnect, got %v", packets)
	}
}

func TestDispatchPollingAuthDoesNotReopenWithActiveWebSocket(t *testing.T) {
	ResetSessionsForTest()
	opener := &countingOpener{}
	h := NewWithOptions(HandlerOptions{
		Version:  "9.3.4-hotfix.1",
		OpenHook: opener,
		CacheDir: t.TempDir(),
	})
	sess := getSession(defaultSessionID, "key", h.Build, h.BasePath)
	sess.mu.Lock()
	sess.transport = noopTransport{}
	sess.docOpenDone = true
	sess.openedKey = "key"
	sess.mu.Unlock()

	connectAuth := `40{"data":{"type":"auth","docid":"key","sessionId":"existing","user":{"id":"demo"},"openCmd":{"c":"open","id":"key","format":"xlsm","url":"http://localhost/f.xlsm"}}}`
	h.dispatchPackets(sess, []string{connectAuth}, "key", httptest.NewRequest("POST", "/?EIO=4&transport=polling&sid=go-office", nil))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if opener.callCount() > 0 {
			t.Fatalf("open calls = %d, want 0 while websocket is active", opener.callCount())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDispatchPollingAuthReopensForFreshEditor(t *testing.T) {
	ResetSessionsForTest()
	opener := &countingOpener{}
	h := NewWithOptions(HandlerOptions{
		Version:  "9.3.4-hotfix.1",
		OpenHook: opener,
		CacheDir: t.TempDir(),
	})
	sess := getSession(defaultSessionID, "key", h.Build, h.BasePath)
	sess.mu.Lock()
	sess.transport = &wsTransport{closed: true}
	sess.docOpenDone = true
	sess.openedKey = "key"
	sess.mu.Unlock()

	connectAuth := `40{"data":{"type":"auth","docid":"key","sessionId":null,"user":{"id":"demo"},"openCmd":{"c":"open","id":"key","format":"csv","url":"http://localhost/f.csv"}}}`
	h.dispatchPackets(sess, []string{connectAuth}, "key", httptest.NewRequest("POST", "/?EIO=4&transport=polling&sid=go-office", nil))

	waitForOpenComplete(t, opener, 1)
}
