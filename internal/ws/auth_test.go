package ws

import "testing"

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
