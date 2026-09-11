package ws_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/ws"
)

func TestDeferredAuthDeliveredWithDocumentOpen(t *testing.T) {
	ws.ResetSessionsForTest()
	opener := newHookOpener()
	h := ws.NewWithOptions(ws.HandlerOptions{
		Version:  "9.3.4",
		CacheDir: t.TempDir(),
		OpenHook: opener,
	})
	h.PollHold = 0
	docKey := "bundle-key"
	docURL := "http://localhost/sample.csv"

	connect := `40{"data":{"type":"auth","docid":"` + docKey + `","user":{"id":"demo","username":"Demo"},"openCmd":{"c":"open","id":"` + docKey + `","format":"csv","url":"` + docURL + `"}}}`
	h.ServePath(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(connect)), "/doc/"+docKey+"/c")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		body := pollingGet(t, h, docKey, "go-office")
		hasAuth := strings.Contains(body, `"type":"auth"`)
		hasOpen := strings.Contains(body, `"type":"documentOpen"`)
		if hasAuth && hasOpen {
			return
		}
		if hasAuth && !hasOpen {
			t.Fatalf("auth arrived without documentOpen in same poll: %q", body)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("auth and documentOpen were not delivered together")
}
