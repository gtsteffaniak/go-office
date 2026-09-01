//go:build linux

package office_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	office "github.com/quantumx-apps/go-office/pkg/office"
	"github.com/quantumx-apps/go-office/internal/testutil"
)

func TestHandleConverterJSON(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	sample := filepath.Join(repo, "sample-files", "sample.docx")
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample docx missing")
	}

	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, sample)
	}))
	defer fileSrv.Close()

	store := nopStorage{}
	srv, err := office.New(store, office.Options{
		AssetDir:     assets,
		PublicOrigin: "http://127.0.0.1:8080",
		ConvertLimit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	body := map[string]any{
		"filetype":   "docx",
		"key":        "playwright-docx-thumb",
		"outputtype": "jpg",
		"title":      "sample.docx",
		"url":        fileSrv.URL + "/sample.docx",
		"thumbnail":  map[string]any{"width": 200, "height": 200},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/converter", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var res office.ConverterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.EndConvert || res.FileURL == "" {
		t.Fatalf("response: %+v", res)
	}

	getReq := httptest.NewRequest(http.MethodGet, res.FileURL, nil)
	getRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("file get status %d url %s", getRec.Code, res.FileURL)
	}
	b := getRec.Body.Bytes()
	if len(b) < 100 || !isRasterImage(b) {
		t.Fatalf("expected image, got %d bytes start %02x %02x", len(b), b[0], b[1])
	}
}

func isRasterImage(raw []byte) bool {
	if len(raw) < 4 {
		return false
	}
	if raw[0] == 0xff && raw[1] == 0xd8 {
		return true
	}
	return raw[0] == 0x89 && raw[1] == 0x50 && raw[2] == 0x4e && raw[3] == 0x47
}
