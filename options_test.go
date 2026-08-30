package office_test

import (
	"testing"

	office "github.com/quantumx-apps/go-office"
)

func TestJoinBasePath(t *testing.T) {
	cases := []struct {
		app, mount, want string
	}{
		{"/", "", "/office"},
		{"/myapp/", "", "/myapp/office"},
		{"/myapp", "office", "/myapp/office"},
		{"", "office", "/office"},
		{"/fb/", "/office/", "/fb/office"},
	}
	for _, tc := range cases {
		got := office.JoinBasePath(tc.app, tc.mount)
		if got != tc.want {
			t.Fatalf("JoinBasePath(%q, %q) = %q, want %q", tc.app, tc.mount, got, tc.want)
		}
	}
}

func TestDefaultBasePath(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if srv.BasePath() != office.DefaultBasePath {
		t.Fatalf("BasePath = %q, want %q", srv.BasePath(), office.DefaultBasePath)
	}
}
