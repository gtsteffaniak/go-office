package netutil

import (
	"net/http/httptest"
	"testing"
)

func TestRequestOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost:8081/demo/config", nil)
	req.Host = "localhost:8081"
	if got := RequestOrigin(req); got != "http://localhost:8081" {
		t.Fatalf("RequestOrigin() = %q, want http://localhost:8081", got)
	}

	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "docs.example.com")
	if got := RequestOrigin(req); got != "https://docs.example.com" {
		t.Fatalf("forwarded RequestOrigin() = %q", got)
	}
}
