package ws

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countingOpener struct {
	mu    sync.Mutex
	calls []string
	err   error
	delay time.Duration
}

func (o *countingOpener) Open(_ context.Context, _, _, docKey string, _ openCmd) ([]string, error) {
	if o.delay > 0 {
		time.Sleep(o.delay)
	}
	o.mu.Lock()
	o.calls = append(o.calls, docKey)
	err := o.err
	o.mu.Unlock()
	if err != nil {
		return nil, err
	}
	pkt, err := documentOpenPacket("open", "ok", map[string]string{"Editor.bin": "http://test/bin"})
	if err != nil {
		return nil, err
	}
	return []string{pkt}, nil
}

func (o *countingOpener) callCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.calls)
}

func TestSessionReconnectSkipsReopen(t *testing.T) {
	ResetSessionsForTest()
	opener := &countingOpener{}
	sess := getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")
	req := authRequest{
		Open: &openCmd{Command: "open", Format: "rtf", URL: "http://localhost/f.rtf"},
		User: authUser{ID: "demo-user", Username: "Demo"},
	}

	sess.startOpen(opener, req, "http://localhost")
	waitForOpenComplete(t, opener, 1)
	_ = sess.drain()

	sess.mu.Lock()
	sess.sessionID = "reconnect-session-id"
	sess.mu.Unlock()

	reconnectReq := authRequest{
		Type:      "auth",
		DocID:     "doc-key",
		SessionID: "reconnect-session-id",
		User:      authUser{ID: "demo-user", Username: "Demo"},
		Open:      &openCmd{Command: "open", Format: "rtf", URL: "http://localhost/f.rtf"},
	}
	raw := connectAuthData(`40{"data":{"type":"auth","docid":"doc-key","sessionId":"reconnect-session-id","user":{"id":"demo-user","username":"Demo"},"openCmd":{"c":"open","id":"doc-key","format":"rtf","url":"http://localhost/f.rtf"}}}`)
	sess.onConnect(raw)
	sess.startOpen(opener, reconnectReq, "http://localhost")
	time.Sleep(20 * time.Millisecond)

	if opener.callCount() != 1 {
		t.Fatalf("reconnect should not reopen document, calls = %d", opener.callCount())
	}
	packets := sess.drain()
	if authCount(packets) != 1 {
		t.Fatalf("reconnect must resend auth, got %d packets=%v", authCount(packets), packets)
	}
	for _, p := range packets {
		if containsType(p, `"type":"documentOpen"`) {
			t.Fatalf("reconnect should not send documentOpen: %v", packets)
		}
	}
}

func TestSessionReconnectWithoutSessionIDSkipsReopen(t *testing.T) {
	ResetSessionsForTest()
	opener := &countingOpener{}
	sess := getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")
	req := authRequest{
		Open: &openCmd{Command: "open", Format: "rtf", URL: "http://localhost/f.rtf"},
		User: authUser{ID: "demo-user", Username: "Demo"},
	}

	sess.startOpen(opener, req, "http://localhost")
	waitForOpenComplete(t, opener, 1)
	_ = sess.drain()

	sess.mu.Lock()
	sess.sessionID = "reconnect-session-id"
	sess.mu.Unlock()

	raw := connectAuthData(`40{"data":{"type":"auth","docid":"doc-key","user":{"id":"demo-user","username":"Demo"},"openCmd":{"c":"open","id":"doc-key","format":"rtf","url":"http://localhost/f.rtf"}}}`)
	sess.onConnect(raw)
	sess.startOpen(opener, authRequest{Open: req.Open}, "http://localhost")
	time.Sleep(20 * time.Millisecond)

	if opener.callCount() != 1 {
		t.Fatalf("reconnect without sessionId should not reopen, calls = %d", opener.callCount())
	}
}

func TestSessionReopenSameDocKey(t *testing.T) {
	ResetSessionsForTest()
	opener := &countingOpener{}
	sess := getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")
	req := authRequest{
		Open: &openCmd{Command: "open", Format: "csv", URL: "http://localhost/f.csv"},
	}

	sess.startOpen(opener, req, "http://localhost")
	waitForOpenComplete(t, opener, 1)

	sess.startOpen(opener, req, "http://localhost")
	if opener.callCount() != 1 {
		t.Fatalf("duplicate startOpen should not reopen, calls = %d", opener.callCount())
	}

	ClearDocumentSession("doc-key")
	sess = getSession(defaultSessionID, "doc-key", ParseBuild("9.3.4"), "")
	sess.onConnect(nil)
	sess.startOpen(opener, req, "http://localhost")
	waitForOpenComplete(t, opener, 2)

	if opener.callCount() != 2 {
		t.Fatalf("open calls after reconnect = %d, want 2", opener.callCount())
	}
}

func TestSessionSwitchIndependentDocKeys(t *testing.T) {
	ResetSessionsForTest()
	opener := &countingOpener{}
	csv := getSession(defaultSessionID, "csv-key", ParseBuild("9.3.4"), "")
	docx := getSession(defaultSessionID, "docx-key", ParseBuild("9.3.4"), "")

	csv.startOpen(opener, authRequest{Open: &openCmd{Command: "open", Format: "csv"}}, "http://localhost")
	waitForOpenComplete(t, opener, 1)

	docx.startOpen(opener, authRequest{Open: &openCmd{Command: "open", Format: "docx"}}, "http://localhost")
	waitForOpenComplete(t, opener, 2)

	csv.onConnect(nil)
	csv.startOpen(opener, authRequest{Open: &openCmd{Command: "open", Format: "csv"}}, "http://localhost")
	waitForOpenComplete(t, opener, 3)

	if opener.callCount() != 3 {
		t.Fatalf("open calls = %d, want 3 across csv/docx switch with csv reload", opener.callCount())
	}
}

func TestSessionAuthDedup(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "key", ParseBuild("9.3.4"), "")
	req := authRequest{User: authUser{ID: "u1", Username: "User"}}

	sess.onAuth(req)
	sess.onAuth(req)

	packets := sess.drain()
	authCount := 0
	for _, p := range packets {
		if containsType(p, `"type":"auth"`) {
			authCount++
		}
	}
	if authCount != 1 {
		t.Fatalf("auth packets = %d, want 1; body=%v", authCount, packets)
	}
}

func TestSessionConnectReloadResendsAuth(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "csv-key", ParseBuild("9.3.4"), "")
	raw := connectAuthData(`40{"data":{"type":"auth","docid":"csv-key","user":{"id":"demo-user","username":"Demo"},"openCmd":{"c":"open","id":"csv-key","format":"csv","url":"http://localhost/sample.csv"}}}`)

	sess.onConnect(raw)
	first := sess.drain()
	if authCount(first) != 1 {
		t.Fatalf("first connect auth packets = %d, body=%v", authCount(first), first)
	}

	sess.onConnect(raw)
	second := sess.drain()
	if authCount(second) != 1 {
		t.Fatalf("reload connect must resend auth, got %d packets=%v", authCount(second), second)
	}
}

func authCount(packets []string) int {
	n := 0
	for _, p := range packets {
		if containsType(p, `"type":"auth"`) {
			n++
		}
	}
	return n
}

func TestSessionOutboxConcurrency(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "key", ParseBuild("9.3.4"), "")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			pkt, err := socketMessage(map[string]any{"type": "test", "n": n})
			if err != nil {
				t.Error(err)
				return
			}
			sess.enqueue(pkt)
		}(i)
	}
	wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	packets := sess.waitForPackets(ctx, 0)
	if len(packets) != 20 {
		t.Fatalf("got %d packets, want 20", len(packets))
	}
}

func TestSessionOpenErrorRecovery(t *testing.T) {
	ResetSessionsForTest()
	opener := &countingOpener{err: errTestOpen}
	sess := getSession(defaultSessionID, "key", ParseBuild("9.3.4"), "")
	req := authRequest{Open: &openCmd{Command: "open", Format: "docx", URL: "http://x"}}

	sess.startOpen(opener, req, "http://localhost")
	waitForOpenComplete(t, opener, 1)

	opener.mu.Lock()
	opener.err = nil
	opener.mu.Unlock()
	sess.startOpen(opener, req, "http://localhost")
	waitForOpenComplete(t, opener, 2)

	if opener.callCount() != 2 {
		t.Fatalf("calls = %d, want 2 after recovery", opener.callCount())
	}
}

var errTestOpen = errors.New("open failed")

func waitForOpenComplete(t *testing.T, opener *countingOpener, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if opener.callCount() >= want {
			time.Sleep(10 * time.Millisecond)
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d open calls, got %d", want, opener.callCount())
}

func containsType(packet, typ string) bool {
	return len(packet) > 0 && (packet == typ || len(packet) > len(typ) && findSubstr(packet, typ))
}

func findSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSessionWaitForPacketsConcurrentPoll(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "key", ParseBuild("9.3.4"), "")

	var ready atomic.Bool
	go func() {
		time.Sleep(20 * time.Millisecond)
		sess.enqueue(`42["message",{"type":"ping"}]`)
		ready.Store(true)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	packets := sess.waitForPackets(ctx, 100*time.Millisecond)
	if !ready.Load() || len(packets) == 0 {
		t.Fatalf("expected packet after enqueue, got %v", packets)
	}
}

func TestSessionStalePollDoesNotStealReloadAuth(t *testing.T) {
	ResetSessionsForTest()
	sess := getSession(defaultSessionID, "csv-key", ParseBuild("9.3.4"), "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := make(chan []string, 1)
	go func() {
		got <- sess.waitForPackets(ctx, 500*time.Millisecond)
	}()
	time.Sleep(20 * time.Millisecond)

	raw := connectAuthData(`40{"data":{"type":"auth","docid":"csv-key","user":{"id":"demo"},"openCmd":{"c":"open","format":"csv","url":"http://localhost/sample.csv"}}}`)
	sess.onConnect(raw)

	stale := <-got
	if authCount(stale) != 0 {
		t.Fatalf("stale long-poll stole reload auth: %v", stale)
	}
	fresh := sess.drain()
	if authCount(fresh) != 1 {
		t.Fatalf("reload auth should remain for the new client, got %v", fresh)
	}
}
