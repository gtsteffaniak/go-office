package debuglog

import (
	"net/http/httptest"
	"testing"
)

func TestIsExpectedHTTPError(t *testing.T) {
	cases := []struct {
		path   string
		status int
		want   bool
	}{
		{"/favicon.ico", 404, true},
		{"/themes.json", 404, true},
		{"/dictionaries/en_US/en_US.aff", 404, true},
		{"/.well-known/appspecific/com.chrome.devtools.json", 404, true},
		{"/missing-api", 404, false},
		{"/themes.json", 500, false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		if got := isExpectedHTTPError(req, tc.status); got != tc.want {
			t.Fatalf("isExpectedHTTPError(%q, %d) = %v, want %v", tc.path, tc.status, got, tc.want)
		}
	}
}
