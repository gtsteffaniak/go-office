package home

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

// DefaultAPIBasePath is where demo API routes (config, callback, file) are mounted.
const DefaultAPIBasePath = "/api/office"

// Options configures the site root landing page.
type Options struct {
	Origin      string // e.g. http://localhost:8080
	OfficeBase  string // e.g. /
	APIBase     string // e.g. /api/office
	SamplesDir  string
	Version     string
	LogoURL     string
	GitHubURL   string
	SamplesOn   bool
}

// Handler serves the about page at /.
type Handler struct {
	tmpl *template.Template
	opts Options
}

// New returns a root handler.
func New(opts Options) (*Handler, error) {
	if opts.APIBase == "" {
		opts.APIBase = DefaultAPIBasePath
	}
	if opts.SamplesDir == "" {
		opts.SamplesDir = "sample-files"
	}
	if opts.GitHubURL == "" {
		opts.GitHubURL = "https://github.com/quantumx-apps/go-office"
	}
	tmpl, err := template.New("home").Parse(pageHTML)
	if err != nil {
		return nil, err
	}
	return &Handler{tmpl: tmpl, opts: opts}, nil
}

// ServeHTTP renders the about page. Only GET / is served; other paths return 404.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	origin := strings.TrimSuffix(h.opts.Origin, "/")
	base := strings.TrimSuffix(h.opts.OfficeBase, "/")
	if base == "" {
		base = ""
	}
	healthBase := origin + base
	data := map[string]any{
		"DemoURL":         origin + office.URLPath(h.opts.OfficeBase, "demo") + "/",
		"HealthURL":       healthBase + "/health",
		"HealthCheckURL":  healthBase + "/healthcheck",
		"APIJSURL":        healthBase + "/web-apps/apps/api/documents/api.js",
		"OfficeBase":      h.opts.OfficeBase,
		"APIBase":         h.opts.APIBase,
		"SamplesDir":      h.opts.SamplesDir,
		"Version":         h.opts.Version,
		"LogoURL":         h.opts.LogoURL,
		"GitHubURL":       h.opts.GitHubURL,
		"SamplesOn":       h.opts.SamplesOn,
	}
	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}
