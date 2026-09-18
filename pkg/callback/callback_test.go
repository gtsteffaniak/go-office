package callback_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/pkg/callback"
)

func TestParseAndShouldPersist(t *testing.T) {
	p, err := callback.Parse([]byte(`{"key":"abc","status":2,"url":"http://localhost/saved.docx"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "abc" || p.Status != callback.StatusMustSave || !p.ShouldPersist() {
		t.Fatalf("payload = %+v", p)
	}

	p, err = callback.Parse([]byte(`{"key":"abc","status":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.ShouldPersist() {
		t.Fatal("editing status should not persist")
	}

	p, err = callback.Parse([]byte(`{"status":6}`))
	if err == nil || !strings.Contains(err.Error(), "key") {
		t.Fatalf("expected key error, got %v", err)
	}
}

func TestParseCallbackResponse(t *testing.T) {
	if err := callback.ParseCallbackResponse([]byte(`{"error":0}`)); err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if err := callback.ParseCallbackResponse([]byte(`{"error":1}`)); err == nil {
		t.Fatal("expected integrator error")
	}
	if err := callback.ParseCallbackResponse([]byte(`not-json`)); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestReadRequestAuthorizationJWT(t *testing.T) {
	secret := []byte("cb-secret")
	raw, err := json.Marshal(callback.Payload{Key: "k1", Status: 2, URL: "http://x/saved.docx"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := callback.SignBody(secret, raw)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/callback", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	got, err := callback.ReadRequest(req, secret)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "k1" || got.Status != callback.StatusMustSave {
		t.Fatalf("payload = %+v", got)
	}
}
