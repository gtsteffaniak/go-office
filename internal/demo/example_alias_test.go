package demo_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/demo"
	"github.com/quantumx-apps/go-office/internal/home"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

// newAliasServer builds a server with the demo UI attached, mirroring the real wiring.
func newAliasServer(t *testing.T) *office.Server {
	t.Helper()
	repoRoot := filepath.Join("..", "..")
	sample := demo.DefaultSamplesDir + "/sample.doc"
	if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(sample))); err != nil {
		t.Skipf("repository sample not present: %v", err)
	}
	store := &memStore{root: repoRoot}
	srv, err := office.New(store, office.Options{BasePath: "/office"})
	if err != nil {
		t.Fatal(err)
	}
	if err := demo.Attach(srv, store, demo.Options{
		PublicOrigin: "http://localhost:8080",
		DataRoot:     repoRoot,
		SamplesDir:   demo.DefaultSamplesDir,
		APIBasePath:  home.DefaultAPIBasePath,
	}); err != nil {
		t.Fatal(err)
	}
	return srv
}

// TestExampleAliasServesDemoUI verifies ONLYOFFICE's bundled-test-example path works.
// Upstream ships a test example at /example/ for trying the editors before integration;
// go-office serves its demo app from that path too, with /demo/ remaining canonical.
func TestExampleAliasServesDemoUI(t *testing.T) {
	srv := newAliasServer(t)

	// The bare path redirects to the trailing-slash form, matching /demo/.
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/office/example", nil))
	if rec.Code != http.StatusPermanentRedirect {
		t.Fatalf("GET /office/example status = %d, want 308", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/office/example/" {
		t.Fatalf("redirect Location = %q, want /office/example/", loc)
	}

	// The landing page renders and its links stay on the /example/ alias.
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/office/example/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /office/example/ status = %d body=%q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Choose a file to open in the editor") {
		t.Fatal("expected the demo landing page at /example/")
	}
	if !strings.Contains(body, "/office/example/view?file=") {
		t.Fatal("landing links at /example/ must stay on the /example/ alias")
	}
	if strings.Contains(body, "/office/demo/view?file=") {
		t.Fatal("landing links at /example/ must not leak /demo/ paths")
	}
}

// TestDemoAliasStillCanonical verifies /demo/ is unchanged by the alias.
func TestDemoAliasStillCanonical(t *testing.T) {
	srv := newAliasServer(t)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/office/demo/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /office/demo/ status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/office/demo/view?file=") {
		t.Fatal("landing links at /demo/ must stay on the /demo/ alias")
	}
}

// TestExampleViewerUsesAliasBase checks the viewer page served from /example/ points its own
// sub-resource calls at /example/, so a user browsing that alias is never bounced to /demo/.
func TestExampleViewerUsesAliasBase(t *testing.T) {
	srv := newAliasServer(t)
	sample := demo.DefaultSamplesDir + "/sample.doc"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/office/example/view?file="+sample, nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /office/example/view status = %d body=%q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"/office/example"`) {
		t.Fatalf("viewer at /example/ should expose uiBase=/office/example, body=%q",
			body[:min(400, len(body))])
	}
	if !strings.Contains(body, `uiBase + "/warm?file="`) {
		t.Fatal("viewer warm call should be built from uiBase")
	}
}
