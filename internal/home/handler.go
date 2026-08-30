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

// DefaultLogoPath is the ONLYOFFICE attribution logo under web-apps assets.
const DefaultLogoPath = "web-apps/apps/common/main/resources/img/about/logo_s.svg"

// Options configures the site root landing page.
type Options struct {
	OfficeBase string // e.g. /
	APIBase    string // e.g. /api/office
	SamplesDir string
	Version    string
	LogoURL    string // relative URL; defaults to DefaultLogoPath under OfficeBase
	GitHubURL  string
	SamplesOn  bool
}

// Handler serves the about page at / and API reference at /docs/api.
type Handler struct {
	homeTmpl *template.Template
	apiTmpl  *template.Template
	opts     Options
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
	if opts.LogoURL == "" {
		opts.LogoURL = office.URLPath(opts.OfficeBase, DefaultLogoPath)
	}
	homeTmpl, err := template.New("home").Parse(pageHTML)
	if err != nil {
		return nil, err
	}
	apiTmpl, err := template.New("apidocs").Parse(apiDocsHTML)
	if err != nil {
		return nil, err
	}
	return &Handler{homeTmpl: homeTmpl, apiTmpl: apiTmpl, opts: opts}, nil
}

// ServeHTTP renders the about page. Only GET / is served; other paths return 404.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	h.render(w, h.homeTmpl)
}

// ServeAPIDocs renders the static API reference at /docs/api.
func (h *Handler) ServeAPIDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.render(w, h.apiTmpl)
}

func (h *Handler) render(w http.ResponseWriter, tmpl *template.Template) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, h.pageData()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) pageData() map[string]any {
	base := h.opts.OfficeBase
	apiBase := strings.TrimSuffix(h.opts.APIBase, "/")
	demoAPI := apiBase + "/demo"
	return map[string]any{
		"DemoURL":           office.URLPath(base, "demo") + "/",
		"HealthURL":         office.URLPath(base, "health"),
		"HealthCheckURL":    office.URLPath(base, "healthcheck"),
		"APIJSURL":          office.URLPath(base, "web-apps/apps/api/documents/api.js"),
		"APIDocsURL":        "/docs/api",
		"InfoURL":           office.URLPath(base, "info/info.json"),
		"WebAppsPrefix":     office.URLPath(base, "web-apps"),
		"CoauthoringPrefix": office.URLPath(base, "doc") + "/{key}/c",
		"CachePrefix":       office.URLPath(base, "cache/files"),
		"DownloadFilePrefix": office.URLPath(base, "downloadfile"),
		"DemoConfigURL":     demoAPI + "/config",
		"DemoFilePrefix":    demoAPI + "/file",
		"DemoCallbackURL":   demoAPI + "/callback",
		"OfficeBase":        base,
		"APIBase":           h.opts.APIBase,
		"SamplesDir":        h.opts.SamplesDir,
		"Version":           h.opts.Version,
		"LogoURL":           h.opts.LogoURL,
		"GitHubURL":         h.opts.GitHubURL,
		"SamplesOn":         h.opts.SamplesOn,
	}
}
