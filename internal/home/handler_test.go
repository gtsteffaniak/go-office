package home_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/home"
)

func TestHomePage(t *testing.T) {
	h, err := home.New(home.Options{
		Origin:     "http://localhost:8080",
		OfficeBase: "/office",
		APIBase:    "/api/office",
		SamplesDir: "sample-files",
		SamplesOn:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	for _, want := range []string{
		"go-office",
		"/office/demo/",
		"/office/health",
		"sample-files",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in home page", want)
		}
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/office/demo/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("non-root path status=%d", rec.Code)
	}
}
