package ws_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/ws"
)

func TestMatchCoauthoringPath(t *testing.T) {
	key, ok := ws.Match("9.0.4/doc/my-key/c")
	if !ok || key != "my-key" {
		t.Fatalf("match failed: ok=%v key=%q", ok, key)
	}
	if _, ok := ws.Match("web-apps/foo"); ok {
		t.Fatal("should not match web-apps path")
	}
}

func TestPollingHandshake(t *testing.T) {
	h := ws.New("9.0.4", nil)
	req := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling", nil)
	rec := httptest.NewRecorder()
	h.ServePath(rec, req, "/9.0.4/doc/key/c")

	body := rec.Body.String()
	if !strings.HasPrefix(body, `0{"sid":"go-office"`) {
		t.Fatalf("open packet = %q", body)
	}
}

func TestPollingLicenseMessage(t *testing.T) {
	h := ws.New("9.0.4", nil)
	req := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&t=abc", nil)
	rec := httptest.NewRecorder()
	h.ServePath(rec, req, "/9.0.4/doc/key/c")

	body := rec.Body.String()
	if !strings.HasPrefix(body, `42["message",`) {
		t.Fatalf("license packet = %q", body)
	}
	if !strings.Contains(body, `"type":"license"`) {
		t.Fatalf("missing license type: %q", body)
	}
}
