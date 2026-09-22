package office_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/quantumx-apps/go-office/pkg/callback"
	"github.com/quantumx-apps/go-office/pkg/config"
	"github.com/quantumx-apps/go-office/pkg/office"
)

func TestDownloadFileAcceptsConfigShapedJWT(t *testing.T) {
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 test"))
	}))
	defer fileSrv.Close()

	secret := []byte("download-jwt-secret")
	srv, err := office.New(nopStorage{}, office.Options{
		BasePath:  "/office",
		JWTSecret: secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	const key = "jwt-doc-key"
	docURL := fileSrv.URL + "/sample.pdf"
	if _, err := srv.BuildEditorConfig(context.Background(), config.EditorRequest{
		DocumentKey: key,
		FileType:    "pdf",
		DocumentURL: docURL,
	}); err != nil {
		t.Fatal(err)
	}

	configClaims := map[string]any{
		"document": map[string]any{
			"key":  key,
			"url":  docURL,
			"fileType": "pdf",
		},
	}
	token, err := callback.SignBody(secret, mustJSON(configClaims))
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{"url": docURL, "token": token})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/downloadfile/"+key, strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config token status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestDownloadFileAcceptsCallbackShapedJWT(t *testing.T) {
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("%PDF-1.4"))
	}))
	defer fileSrv.Close()

	secret := []byte("download-jwt-secret")
	srv, err := office.New(nopStorage{}, office.Options{JWTSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	const key = "cb-key"
	docURL := fileSrv.URL + "/sample.pdf"
	if _, err := srv.BuildEditorConfig(context.Background(), config.EditorRequest{
		DocumentKey: key,
		FileType:    "pdf",
		DocumentURL: docURL,
	}); err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(callback.Payload{Key: key, Status: 1, URL: docURL})
	if err != nil {
		t.Fatal(err)
	}
	token, err := callback.SignBody(secret, body)
	if err != nil {
		t.Fatal(err)
	}
	reqBody, _ := json.Marshal(map[string]string{"url": docURL, "token": token})
	req := httptest.NewRequest(http.MethodPost, "/downloadfile/"+key, strings.NewReader(string(reqBody)))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("callback token status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestDownloadFileRejectsWrongJWTSecret(t *testing.T) {
	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer fileSrv.Close()

	srv, err := office.New(nopStorage{}, office.Options{JWTSecret: []byte("server-secret")})
	if err != nil {
		t.Fatal(err)
	}
	const key = "bad-key"
	docURL := fileSrv.URL + "/f"
	if _, err := srv.BuildEditorConfig(context.Background(), config.EditorRequest{
		DocumentKey: key,
		FileType:    "pdf",
		DocumentURL: docURL,
	}); err != nil {
		t.Fatal(err)
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"key": key})
	signed, err := tok.SignedString([]byte("other-secret"))
	if err != nil {
		t.Fatal(err)
	}
	reqBody, _ := json.Marshal(map[string]string{"url": docURL, "token": signed})
	req := httptest.NewRequest(http.MethodPost, "/downloadfile/"+key, strings.NewReader(string(reqBody)))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
