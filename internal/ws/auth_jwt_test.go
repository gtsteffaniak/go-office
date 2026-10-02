package ws_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/quantumx-apps/go-office/internal/ws"
)

// jwtTestHandler builds a handler that requires JWT verification.
func jwtTestHandler(t *testing.T, secret string) *ws.Handler {
	t.Helper()
	ws.ResetSessionsForTest()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:   "9.3.4",
		CacheDir:  t.TempDir(),
		JWTSecret: []byte(secret),
		OpenHook:  newHookOpener(),
	})
	h.PollHold = 0
	return h
}

func signedConfigToken(t *testing.T, secret, docKey string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"document": map[string]any{"key": docKey, "fileType": "xlsx"},
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func connectPacket(docKey, token string) string {
	return `40{"data":{"type":"auth","docid":"` + docKey + `","token":"` + token + `",` +
		`"user":{"id":"demo","username":"Demo"},` +
		`"openCmd":{"c":"open","id":"` + docKey + `","format":"xlsx","url":"http://localhost/s.xlsx"}}}`
}

// TestRejectedJWTClosesSession is the security property: when JWT verification is enabled and
// the token is bad, the session must be REFUSED, not opened unverified.
//
// Previously the handler logged a warning and called onConnect anyway, so the document opened
// without verification — the check looked enforced but provided no protection.
func TestRejectedJWTClosesSession(t *testing.T) {
	const secret = "test-secret"
	docKey := "jwt-reject-key"
	h := jwtTestHandler(t, secret)

	// The vanilla sdkjs placeholder is a plain string, not a JWT.
	body := connectPacket(docKey, "fghhfgsjdgfjs")
	h.ServePath(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body)),
		"/doc/"+docKey+"/c")

	var out string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out += pollingGet(t, h, docKey, "go-office")
		if strings.Contains(out, `"type":"close"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !strings.Contains(out, `"type":"close"`) {
		t.Fatalf("expected a close packet for a rejected JWT, got %q", out)
	}
	// 4006 = c_oCloseCode.jwtError, which sdkjs surfaces as an editor error.
	if !strings.Contains(out, "4006") {
		t.Fatalf("expected close code 4006 (jwtError), got %q", out)
	}
	if strings.Contains(out, `"type":"documentOpen"`) {
		t.Fatalf("document must not open with a rejected JWT: %q", out)
	}
}

// TestValidJWTOpensDocument confirms the rejection path does not break valid sessions.
func TestValidJWTOpensDocument(t *testing.T) {
	const secret = "test-secret"
	docKey := "jwt-accept-key"
	h := jwtTestHandler(t, secret)

	body := connectPacket(docKey, signedConfigToken(t, secret, docKey))
	h.ServePath(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body)),
		"/doc/"+docKey+"/c")

	var out string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out += pollingGet(t, h, docKey, "go-office")
		if strings.Contains(out, `"type":"documentOpen"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if strings.Contains(out, `"type":"close"`) {
		t.Fatalf("valid JWT must not be closed: %q", out)
	}
	if !strings.Contains(out, `"type":"documentOpen"`) {
		t.Fatalf("expected the document to open with a valid JWT, got %q", out)
	}
}

// TestMissingJWTFailsClosed ensures enabling JWT without a token refuses the session.
func TestMissingJWTFailsClosed(t *testing.T) {
	h := jwtTestHandler(t, "test-secret")
	docKey := "jwt-missing-key"

	body := `40{"data":{"type":"auth","docid":"` + docKey + `","user":{"id":"demo","username":"Demo"},` +
		`"openCmd":{"c":"open","id":"` + docKey + `","format":"xlsx","url":"http://localhost/s.xlsx"}}}`
	h.ServePath(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body)),
		"/doc/"+docKey+"/c")

	var out string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out += pollingGet(t, h, docKey, "go-office")
		if strings.Contains(out, `"type":"close"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(out, `"type":"close"`) {
		t.Fatalf("missing JWT must fail closed, got %q", out)
	}
}

// TestJWTDisabledAllowsPlainToken preserves the no-secret deployment: verification is off, so
// a placeholder token must not close the session.
func TestJWTDisabledAllowsPlainToken(t *testing.T) {
	ws.ResetSessionsForTest()
	docKey := "jwt-disabled-key"
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: newHookOpener(),
	})
	h.PollHold = 0

	body := connectPacket(docKey, "fghhfgsjdgfjs")
	h.ServePath(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(body)),
		"/doc/"+docKey+"/c")

	var out string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out += pollingGet(t, h, docKey, "go-office")
		if strings.Contains(out, `"type":"documentOpen"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Contains(out, `"type":"close"`) {
		t.Fatalf("JWT disabled must not close the session: %q", out)
	}
	if !strings.Contains(out, `"type":"documentOpen"`) {
		t.Fatalf("expected document to open with JWT disabled, got %q", out)
	}
}
