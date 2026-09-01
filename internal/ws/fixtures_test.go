package ws_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/ws"
)

type fixtureCase struct {
	Name        string   `json:"name"`
	Post        string   `json:"post"`
	ExpectTypes []string `json:"expectTypes"`
}

func TestCoauthoringGoldenFixtures(t *testing.T) {
	ws.ResetSessionsForTest()
	raw, err := os.ReadFile(filepath.Join("fixtures", "coauthoring.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []fixtureCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ws.ResetSessionsForTest()
			h := ws.NewWithOptions(ws.HandlerOptions{
				Version:  "9.3.4-hotfix.1",
				CacheDir: t.TempDir(),
				Saver:    nopSaver{},
			})
			h.PollHold = 0

			if strings.HasPrefix(tc.Post, "42") {
				post := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(tc.Post))
				h.ServePath(httptest.NewRecorder(), post, "/doc/key/c")
			} else {
				post := httptest.NewRequest(http.MethodPost, "/?EIO=4&transport=polling&sid=go-office", strings.NewReader(tc.Post))
				h.ServePath(httptest.NewRecorder(), post, "/doc/key/c")
			}

			get := httptest.NewRequest(http.MethodGet, "/?EIO=4&transport=polling&sid=go-office&t=1", nil)
			rec := httptest.NewRecorder()
			h.ServePath(rec, get, "/doc/key/c")
			body := rec.Body.String()
			if tc.Name == "handshake" && !strings.Contains(body, `40{"sid":"go-office"}`) {
				t.Fatalf("missing namespace ack in %q", body)
			}
			for _, typ := range tc.ExpectTypes {
				if !strings.Contains(body, `"type":"`+typ+`"`) {
					t.Fatalf("missing type %q in %q", typ, body)
				}
			}
		})
	}
}
