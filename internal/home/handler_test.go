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
		`href="/office/demo/"`,
		`href="/office/health"`,
		`href="/docs/api"`,
		"/doc/{key}/c/",
		"sample-files",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in home page", want)
		}
	}
	if strings.Contains(body, "http://localhost") {
		t.Fatalf("home page should use relative URLs, got: %s", body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/office/demo/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("non-root path status=%d", rec.Code)
	}
}

func TestAPIDocsPage(t *testing.T) {
	h, err := home.New(home.Options{
		OfficeBase: "/",
		APIBase:    "/api/office",
		SamplesOn:  true,
		Version:    "9.3.4",
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeAPIDocs(rec, httptest.NewRequest(http.MethodGet, "/docs/api", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	for _, want := range []string{
		"Document Server API",
		`href="/health"`,
		`href="/api/office/demo/config"`,
		"saveChanges",
		"getLock",
		"forceSaveStart",
		"/doc/{key}/c/",
		"id=\"compatibility\"",
		"Coauthoring WebSocket",
		"POST /command",
		"Verdict",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in api docs", want)
		}
	}
}
