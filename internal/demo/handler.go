package demo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/quantumx-apps/go-office/internal/home"
	"github.com/quantumx-apps/go-office/internal/netutil"
	"github.com/quantumx-apps/go-office/pkg/config"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

// Options configures the local demo UI and API stubs.
type Options struct {
	// PublicOrigin overrides request host when set (reverse proxy). Leave empty for same-origin demo.
	PublicOrigin string
	DataRoot     string // filesystem root for listing sample documents
	SamplesDir   string // directory relative to DataRoot (default: sample-files)
	APIBasePath  string // API routes prefix (default: /api/office)
	Logger       *slog.Logger
}

// Handler serves a minimal editor page plus config/file/callback routes for local testing.
type Handler struct {
	office *office.Server
	store  office.Storage
	opts   Options

	landingTmpl *template.Template
	viewerTmpl  *template.Template
}

type landingFile struct {
	Name         string
	URL          string
	ThumbnailURL string
	Size         string
}

type landingData struct {
	SamplesDir string
	Files      []landingFile
}

type viewerData struct {
	FileJSON       template.JS
	OfficeBaseJSON template.JS
	APIBaseJSON    template.JS
	LandingURLJSON template.JS
	LandingURL     string
}

// New returns a demo handler. Register patterns with Attach.
func New(srv *office.Server, store office.Storage, opts Options) (*Handler, error) {
	if opts.SamplesDir == "" {
		opts.SamplesDir = DefaultSamplesDir
	}
	if opts.APIBasePath == "" {
		opts.APIBasePath = home.DefaultAPIBasePath
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	landingTmpl, err := template.New("landing").Parse(landingHTML)
	if err != nil {
		return nil, fmt.Errorf("demo: landing template: %w", err)
	}
	viewerTmpl, err := template.New("viewer").Parse(viewerHTML)
	if err != nil {
		return nil, fmt.Errorf("demo: viewer template: %w", err)
	}

	return &Handler{
		office:      srv,
		store:       store,
		opts:        opts,
		landingTmpl: landingTmpl,
		viewerTmpl:  viewerTmpl,
	}, nil
}

// Attach registers demo UI routes under the office base path and API routes under APIBasePath.
func Attach(srv *office.Server, store office.Storage, opts Options) error {
	h, err := New(srv, store, opts)
	if err != nil {
		return err
	}
	uiBase := office.URLPath(srv.BasePath(), "demo")
	srv.Mount(uiBase, http.RedirectHandler(uiBase+"/", http.StatusPermanentRedirect))
	srv.Mount(uiBase+"/", http.StripPrefix(uiBase, http.HandlerFunc(h.serveUI)))

	apiBase := normalizePath(opts.APIBasePath) + "/demo"
	srv.Mount(apiBase, http.RedirectHandler(apiBase+"/", http.StatusPermanentRedirect))

	api := http.NewServeMux()
	api.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) {
		h.serveConfig(w, r)
	})
	api.HandleFunc("GET /file/{path...}", func(w http.ResponseWriter, r *http.Request) {
		h.serveFile(w, r)
	})
	api.HandleFunc("POST /callback", func(w http.ResponseWriter, r *http.Request) {
		h.serveCallback(w, r)
	})
	api.HandleFunc("GET /thumbnail", func(w http.ResponseWriter, r *http.Request) {
		h.serveThumbnail(w, r)
	})
	srv.Mount(apiBase+"/", http.StripPrefix(apiBase, api))
	return nil
}

func (h *Handler) serveUI(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && (r.URL.Path == "" || r.URL.Path == "/"):
		h.serveLanding(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/view":
		h.serveViewer(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) serveConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	file := strings.TrimSpace(r.URL.Query().Get("file"))
	if file == "" {
		http.Error(w, "file query parameter is required", http.StatusBadRequest)
		return
	}
	if !h.isAllowedSample(file) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	info, err := h.store.Stat(ctx, file)
	if err != nil {
		http.Error(w, "sample file not found: "+file, http.StatusNotFound)
		return
	}

	origin := strings.TrimSuffix(netutil.RequestOrigin(r), "/")
	if o := strings.TrimSpace(h.opts.PublicOrigin); o != "" {
		origin = strings.TrimSuffix(o, "/")
	}
	apiBase := h.opts.APIBasePath
	fileURL := origin + apiBase + "/demo/file/" + strings.TrimPrefix(file, "/")
	callbackURL := origin + apiBase + "/demo/callback"
	key := documentKey(file, info, h.fileFingerprint(r.Context(), file, info))
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(info.Name)), ".")

	cfg, err := h.office.BuildEditorConfig(ctx, config.EditorRequest{
		DocumentKey: key,
		Title:       info.Name,
		FileType:    ext,
		DocumentURL: fileURL,
		StoragePath: file,
		CallbackURL: callbackURL,
		UserID:      "demo-user",
		UserName:    "Demo User",
		Mode:        "edit",
		Lang:        "en",
		Theme:       "light",
		Permissions: config.Permissions{Edit: "edit", Download: true, Print: true},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.office.Debug() {
		h.opts.Logger.Debug("demo config",
			"file", file,
			"key", key,
			"documentURL", fileURL,
			"callbackURL", callbackURL,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

func (h *Handler) serveViewer(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimSpace(r.URL.Query().Get("file"))
	if file == "" {
		http.Redirect(w, r, ".", http.StatusFound)
		return
	}
	if !h.isAllowedSample(file) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	officeBase := h.office.BasePath()
	if officeBase == "/" {
		officeBase = ""
	}
	apiBase := normalizePath(h.opts.APIBasePath)
	landingURL := office.URLPath(h.office.BasePath(), "demo/")

	data := viewerData{
		FileJSON:       template.JS(jsonString(file)),
		OfficeBaseJSON: template.JS(jsonString(officeBase)),
		APIBaseJSON:    template.JS(jsonString(apiBase)),
		LandingURLJSON: template.JS(jsonString(landingURL)),
		LandingURL:     landingURL,
	}
	var buf bytes.Buffer
	if err := h.viewerTmpl.Execute(&buf, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.PathValue("path"), "/")
	if rel == "" || !h.isAllowedSample(rel) {
		http.NotFound(w, r)
		return
	}

	if h.office.Debug() {
		h.opts.Logger.Debug("demo file", "path", rel)
	}

	rc, err := h.store.Open(r.Context(), rel)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer rc.Close()

	name := path.Base(rel)
	ctype := mime.TypeByExtension(path.Ext(name))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = io.Copy(w, rc)
}

func (h *Handler) serveCallback(w http.ResponseWriter, r *http.Request) {
	h.office.HandleCallback(w, r)
}

func (h *Handler) serveThumbnail(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimSpace(r.URL.Query().Get("file"))
	if file == "" || !h.isAllowedSample(file) {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	info, err := h.store.Stat(ctx, file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	origin := strings.TrimSuffix(netutil.RequestOrigin(r), "/")
	if o := strings.TrimSpace(h.opts.PublicOrigin); o != "" {
		origin = strings.TrimSuffix(o, "/")
	}
	apiBase := h.opts.APIBasePath
	fileURL := origin + apiBase + "/demo/file/" + strings.TrimPrefix(file, "/")
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(info.Name)), ".")
	key := documentKey(file, info, h.fileFingerprint(ctx, file, info))
	req := office.ConverterRequest{
		FileType:   ext,
		Key:        key,
		OutputType: "jpg",
		Title:      info.Name,
		URL:        fileURL,
		Thumbnail:  &office.ConverterThumbnail{Width: 200, Height: 200, Aspect: 2, First: true},
	}
	if _, runErr := h.office.RunConverter(ctx, origin, req); runErr != nil {
		if h.opts.Logger != nil {
			h.opts.Logger.Error("demo thumbnail", "file", file, "err", runErr)
		}
		http.Error(w, "thumbnail failed", http.StatusInternalServerError)
		return
	}
	cacheName := office.ConvCacheDirName(key, "jpg")
	outPath := filepath.Join(h.office.CacheDir(), cacheName, office.ConvOutputBasename("jpg", req.Thumbnail))
	raw, err := os.ReadFile(outPath)
	if err != nil {
		http.Error(w, "thumbnail missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(raw)
}

func (h *Handler) serveLanding(w http.ResponseWriter, _ *http.Request) {
	files, err := h.listSampleFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := landingData{
		SamplesDir: h.opts.SamplesDir,
		Files:      files,
	}
	var buf bytes.Buffer
	if err := h.landingTmpl.Execute(&buf, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) listSampleFiles() ([]landingFile, error) {
	root := filepath.Join(h.opts.DataRoot, filepath.FromSlash(h.opts.SamplesDir))
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("samples directory not found: %s", h.opts.SamplesDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("samples path is not a directory: %s", h.opts.SamplesDir)
	}

	var files []landingFile
	err = filepath.WalkDir(root, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !isSupportedDocument(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(h.opts.DataRoot, full)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.Contains(rel, "/playwright/") {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		viewPath := office.URLPath(h.office.BasePath(), "demo/view")
		thumbURL := h.opts.APIBasePath + "/demo/thumbnail?file=" + url.QueryEscape(rel)
		files = append(files, landingFile{
			Name:         rel,
			URL:          viewPath + "?file=" + url.QueryEscape(rel),
			ThumbnailURL: thumbURL,
			Size:         formatSize(fi.Size()),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	return files, nil
}

func (h *Handler) isAllowedSample(rel string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	if rel == "" || strings.Contains(rel, "..") {
		return false
	}
	prefix := strings.TrimSuffix(filepath.ToSlash(h.opts.SamplesDir), "/") + "/"
	if !strings.HasPrefix(rel, prefix) {
		return false
	}
	name := path.Base(rel)
	return isSupportedDocument(name)
}

func isSupportedDocument(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".doc", ".docx", ".dot", ".dotx", ".odt", ".rtf", ".txt",
		".xls", ".xlsx", ".xlsm", ".ods", ".csv",
		".ppt", ".pptx", ".pptm", ".odp",
		".pdf":
		return true
	default:
		return false
	}
}

func documentKey(file string, info office.FileInfo, contentSum string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%s", file, info.Size, info.ModTime.UnixNano(), contentSum)))
	return hex.EncodeToString(sum[:])
}

func (h *Handler) fileFingerprint(ctx context.Context, file string, info office.FileInfo) string {
	if h == nil || h.store == nil {
		return fmt.Sprintf("%d:%d", info.Size, info.ModTime.UnixNano())
	}
	rc, err := h.store.Open(ctx, file)
	if err != nil {
		return fmt.Sprintf("%d:%d", info.Size, info.ModTime.UnixNano())
	}
	defer rc.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, io.LimitReader(rc, 32<<20)); err != nil {
		return fmt.Sprintf("%d:%d", info.Size, info.ModTime.UnixNano())
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return home.DefaultAPIBasePath
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
