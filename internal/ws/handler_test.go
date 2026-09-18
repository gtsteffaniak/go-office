package ws_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/ws"
)

func testHandler(t *testing.T) *ws.Handler {
	t.Helper()
	ws.ResetSessionsForTest()
	h := ws.New("9.3.4-hotfix.1", nil)
	h.PollHold = 0
	return h
}

func TestMatchCoauthoringPath(t *testing.T) {
	cases := []struct {
		path string
		key  string
	}{
		{"9.0.4/doc/my-key/c", "my-key"},
		{"doc/my-key/c", "my-key"},
	}
	for _, tc := range cases {
		key, ok := ws.Match(tc.path)
		if !ok || key != tc.key {
			t.Fatalf("Match(%q) = (%q, %v), want (%q, true)", tc.path, key, ok, tc.key)
		}
	}
	if _, ok := ws.Match("web-apps/foo"); ok {
		t.Fatal("should not match web-apps path")
	}
}

func TestPollingHandshake(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&t=abc", nil)
	rec := httptest.NewRecorder()
	h.ServePath(rec, req, "/doc/key/c")

	body := rec.Body.String()
	if !strings.HasPrefix(body, `0{"sid":"`) {
		t.Fatalf("open packet = %q", body)
	}
}

func TestPollingConnectAndLicense(t *testing.T) {
	h := testHandler(t)

	post := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(`40`))
	postRec := httptest.NewRecorder()
	h.ServePath(postRec, post, "/doc/key/c")
	if postRec.Body.String() != "ok" {
		t.Fatalf("post ack = %q", postRec.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=abc", nil)
	getRec := httptest.NewRecorder()
	h.ServePath(getRec, get, "/doc/key/c")
	body := getRec.Body.String()
	if !strings.Contains(body, `40{"sid":"go-office"}`) {
		t.Fatalf("namespace ack missing: %q", body)
	}
	if !strings.Contains(body, `"type":"license"`) {
		t.Fatalf("server info missing: %q", body)
	}
	if !strings.Contains(body, `"buildVersion":"9.3.4"`) {
		t.Fatalf("buildVersion should match sdkjs (9.3.4): %q", body)
	}
	if !strings.Contains(body, `"buildNumber":0`) {
		t.Fatalf("buildNumber should be integer 0: %q", body)
	}
	if !strings.Contains(body, `"type":3`) {
		t.Fatalf("handshake should report Success (3): %q", body)
	}

	// Second poll with nothing queued returns noop when PollHold=0.
	get2 := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=def", nil)
	getRec2 := httptest.NewRecorder()
	h.ServePath(getRec2, get2, "/doc/key/c")
	if getRec2.Body.String() != "6" {
		t.Fatalf("expected noop, got %q", getRec2.Body.String())
	}
}

func TestPollingAuthResponse(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0

	authBody := `42["message",{"type":"auth","docid":"key","user":{"id":"demo","username":"Demo"},"openCmd":{"c":"open","id":"key","format":"doc","url":"http://localhost/f.doc"}}]`
	authPost := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(authBody))
	h.ServePath(httptest.NewRecorder(), authPost, "/doc/key/c")

	deadline := time.Now().Add(2 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		body = pollingGet(t, h, "key", "go-office")
		if strings.Contains(body, `"type":"auth"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(body, `"type":"authChanges"`) {
		t.Fatalf("authChanges missing: %q", body)
	}
	if !strings.Contains(body, `"type":"auth"`) {
		t.Fatalf("auth response missing: %q", body)
	}
	if !strings.Contains(body, `"result":1`) {
		t.Fatalf("auth result missing: %q", body)
	}
	if !strings.Contains(body, `"buildVersion":"9.3.4"`) {
		t.Fatalf("auth buildVersion missing: %q", body)
	}
	if !strings.Contains(body, `"binaryChanges":false`) {
		t.Fatalf("auth should disable binaryChanges: %q", body)
	}
}

func TestPollingReloadSameCSVResendsAuth(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4-hotfix.1",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0

	csvKey := "0c799d3dbda398a50f7077f6f3c3de7cb9110610d27e9318951de50ec9788e47"
	connectAuth := `40{"data":{"type":"auth","docid":"` + csvKey + `","user":{"id":"demo-user","username":"Demo User"},"openCmd":{"c":"open","id":"` + csvKey + `","format":"csv","url":"http://localhost/sample.csv"}}}`

	postReload := func() {
		req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(connectAuth))
		h.ServePath(httptest.NewRecorder(), req, "/doc/"+csvKey+"/c")
	}

	// First load uses Engine.IO packet 40 with embedded auth + openCmd (browser reload shape).
	postReload()
	waitDocumentOpen(t, h, csvKey, "go-office")
	if opener.count(csvKey) != 1 {
		t.Fatalf("first open calls = %d, want 1", opener.count(csvKey))
	}

	// Same hardcoded sid + document key, as the browser does on reload.
	postReload()
	second := pollingGet(t, h, csvKey, "go-office")
	if !strings.Contains(second, `"type":"auth"`) {
		t.Fatalf("reload must resend auth for the same csv key, got %q", second)
	}
	if !strings.Contains(second, `"result":1`) {
		t.Fatalf("reload auth result missing: %q", second)
	}
	if strings.Contains(second, `"type":"documentOpen"`) {
		t.Fatalf("reload without session clear must not send documentOpen: %q", second)
	}
	if opener.count(csvKey) != 1 {
		t.Fatalf("reload without clear should not open again, got %d calls", opener.count(csvKey))
	}

	// Integrator reset (BuildEditorConfig / POST session/reset).
	ws.ClearDocumentSession(csvKey)
	postReload()
	waitDocumentOpen(t, h, csvKey, "go-office")
	if opener.count(csvKey) < 2 {
		t.Fatalf("reload after session reset must open again, got %d calls", opener.count(csvKey))
	}
}

func TestPollingConnectThenAuthOpensOnce(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4-hotfix.1",
		OpenHook: opener,
	})
	h.PollHold = 0

	key := "xlsm-key"
	openBody := `{"c":"open","id":"` + key + `","format":"xlsm","url":"http://localhost/sample.xlsm"}`
	connectAuth := `40{"data":{"type":"auth","docid":"` + key + `","user":{"id":"demo-user","username":"Demo"},"openCmd":` + openBody + `}}`
	authMsg := `42["message",{"type":"auth","docid":"` + key + `","user":{"id":"demo-user","username":"Demo"},"openCmd":` + openBody + `}]`

	post := func(body string) {
		req := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body))
		h.ServePath(httptest.NewRecorder(), req, "/doc/"+key+"/c")
	}
	poll := func() string {
		get := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=1", nil)
		rec := httptest.NewRecorder()
		h.ServePath(rec, get, "/doc/"+key+"/c")
		return rec.Body.String()
	}

	post(connectAuth)
	post(authMsg)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if opener.count(key) >= 1 && strings.Contains(poll(), `"type":"documentOpen"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if opener.count(key) != 1 {
		t.Fatalf("connect+auth should open once, got %d opens", opener.count(key))
	}
}
