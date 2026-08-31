package office_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quantumx-apps/go-office/pkg/callback"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestNotifyCallbackSignsJWT(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		callback.WriteOK(w)
	}))
	defer srv.Close()

	officeSrv, err := office.New(nopStorage{}, office.Options{JWTSecret: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	if err := officeSrv.NotifyCallback(context.Background(), "k1", srv.URL, "http://x/saved.csv", false); err != nil {
		t.Fatal(err)
	}
	var wrapped map[string]string
	if err := json.Unmarshal(gotBody, &wrapped); err != nil {
		t.Fatalf("body: %s", gotBody)
	}
	if wrapped["token"] == "" {
		t.Fatal("expected JWT token in callback body")
	}
}
