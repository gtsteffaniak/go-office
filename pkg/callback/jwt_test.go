package callback_test

import (
	"encoding/json"
	"testing"

	"github.com/quantumx-apps/go-office/pkg/callback"
)

func TestSignAndVerifyBody(t *testing.T) {
	secret := []byte("test-secret")
	body, err := json.Marshal(callback.Payload{Key: "k1", Status: 2, URL: "http://x/saved.docx"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := callback.SignBody(secret, body)
	if err != nil || token == "" {
		t.Fatalf("sign: %v", err)
	}
	wrapped, _ := json.Marshal(map[string]string{"token": token})
	got, err := callback.ParseRequest(wrapped, secret)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "k1" || got.Status != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestVerifySignatureConfigShapedToken(t *testing.T) {
	secret := []byte("test-secret")
	body, err := json.Marshal(map[string]any{
		"document": map[string]any{
			"key": "doc-key",
			"url": "http://example.com/file.csv",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := callback.SignBody(secret, body)
	if err != nil || token == "" {
		t.Fatalf("sign: %v", err)
	}
	claims, err := callback.VerifySignature(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := claims["document"].(map[string]any)
	if !ok || doc["key"] != "doc-key" {
		t.Fatalf("claims document: %+v", claims["document"])
	}
	if _, err := callback.VerifyBody(secret, token); err == nil {
		t.Fatal("VerifyBody should reject config-shaped claims without top-level key")
	}
}

func TestVerifySignatureWrongSecret(t *testing.T) {
	secret := []byte("test-secret")
	token, err := callback.SignBody(secret, []byte(`{"key":"k1","status":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := callback.VerifySignature([]byte("other"), token); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
