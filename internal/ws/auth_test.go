package ws

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthRequestIntegratorCallbackURL(t *testing.T) {
	req := authRequest{DocumentCallbackURL: "http://host/callback?auth=x"}
	if got := req.IntegratorCallbackURL(); got != "http://host/callback?auth=x" {
		t.Fatalf("IntegratorCallbackURL() = %q", got)
	}
	req = authRequest{CallbackURL: "http://host/fallback"}
	if got := req.IntegratorCallbackURL(); got != "http://host/fallback" {
		t.Fatalf("IntegratorCallbackURL() = %q", got)
	}
}

func TestParseAuthPacketDocumentCallbackURL(t *testing.T) {
	packet := `40{"data":{"type":"auth","docid":"doc-key","documentCallbackUrl":"http://host/cb","user":{"id":"u1"},"openCmd":{"c":"open","format":"xlsx","url":"http://host/file"}}}`
	req, ok := parseAuthPacket(packet)
	if !ok {
		t.Fatal("parseAuthPacket failed")
	}
	if req.IntegratorCallbackURL() != "http://host/cb" {
		t.Fatalf("callback URL = %q", req.IntegratorCallbackURL())
	}
	if req.Open == nil || req.Open.Format != "xlsx" {
		t.Fatalf("openCmd = %#v", req.Open)
	}
}

func TestVerifyAuthJWTSkipsWhenSecretUnset(t *testing.T) {
	if err := verifyAuthJWT(nil, "", "doc-key"); err != nil {
		t.Fatalf("expected no error without secret, got %v", err)
	}
}

func TestVerifyAuthJWTRejectsMissingToken(t *testing.T) {
	if err := verifyAuthJWT([]byte("secret"), "", "doc-key"); err == nil {
		t.Fatal("expected missing jwt error")
	}
}

func TestVerifyAuthJWTAcceptsValidToken(t *testing.T) {
	secret := []byte("test-secret")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"document": map[string]any{"key": "doc-key"},
		"exp":      time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyAuthJWT(secret, signed, "doc-key"); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
}

func TestVerifyAuthJWTRejectsKeyMismatch(t *testing.T) {
	secret := []byte("test-secret")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"document": map[string]any{"key": "other-key"},
	})
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyAuthJWT(secret, signed, "doc-key"); err == nil {
		t.Fatal("expected key mismatch error")
	}
}
