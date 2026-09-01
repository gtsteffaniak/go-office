package office_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/quantumx-apps/go-office/internal/session"
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

func TestHandleCallbackSkipsPersistWithoutPendingChanges(t *testing.T) {
	assetDir := t.TempDir()
	store := &recordingStorage{t: t}
	srv, err := office.New(store, office.Options{AssetDir: assetDir})
	if err != nil {
		t.Fatal(err)
	}

	docKey := "txt-callback-skip"
	docCache := filepath.Join(srv.CacheDir(), docKey)
	if err := os.MkdirAll(docCache, 0o755); err != nil {
		t.Fatal(err)
	}

	srv.Sessions().UpsertDoc(session.Document{
		Key:      docKey,
		Path:     "sample-files/sample.txt",
		FileType: "txt",
	})

	body, _ := json.Marshal(map[string]any{
		"key":    docKey,
		"status": 6,
	})
	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.HandleCallback(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if store.savedPath != "" {
		t.Fatalf("expected no storage write without pending changes, got %q", store.savedPath)
	}
}

func TestHandleCallbackDownloadsWhenURLPresent(t *testing.T) {
	download := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte{0xef, 0xbb, 0xbf, 'h', 'i'})
	}))
	defer download.Close()

	store := &recordingStorage{t: t, savedBody: &bytes.Buffer{}}
	srv, err := office.New(store, office.Options{AssetDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	docKey := "txt-callback-download"
	srv.Sessions().UpsertDoc(session.Document{
		Key:      docKey,
		Path:     "sample-files/sample.txt",
		FileType: "txt",
	})

	body, _ := json.Marshal(map[string]any{
		"key":    docKey,
		"status": 6,
		"url":    download.URL + "/saved.txt",
	})
	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.HandleCallback(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if store.savedPath != "sample-files/sample.txt" {
		t.Fatalf("saved path = %q", store.savedPath)
	}
	if got := store.savedBody.String(); got != "hi" {
		t.Fatalf("saved body = %q", got)
	}
}
