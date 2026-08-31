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
