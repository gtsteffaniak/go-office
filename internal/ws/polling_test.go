package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsCoauthoringPollingCheck(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		want   bool
	}{
		{"long poll get", http.MethodGet, "/doc/abc/c/?EIO=4&transport=polling&sid=go-office&t=1", true},
		{"handshake get", http.MethodGet, "/doc/abc/c/?EIO=4&transport=polling", true},
		{"polling post", http.MethodPost, "/doc/abc/c/?EIO=4&transport=polling&sid=go-office", false},
		{"websocket", http.MethodGet, "/doc/abc/c/?transport=websocket", false},
		{"other path", http.MethodGet, "/health", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, query, _ := strings.Cut(tc.target, "?")
			req := httptest.NewRequest(tc.method, path, nil)
			if query != "" {
				req.URL.RawQuery = query
			}
			if got := IsCoauthoringPollingCheck(req); got != tc.want {
				t.Fatalf("IsCoauthoringPollingCheck() = %v, want %v", got, tc.want)
			}
		})
	}
}
