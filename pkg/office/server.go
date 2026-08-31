package office

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"

	"github.com/quantumx-apps/go-office/pkg/config"
	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/debuglog"
	"github.com/quantumx-apps/go-office/internal/session"
	"github.com/quantumx-apps/go-office/internal/static"
	"github.com/quantumx-apps/go-office/internal/ws"
)

// Server is an embedded ONLYOFFICE-compatible document server.
type Server struct {
	storage Storage
	opts    Options

	sessions     *session.Manager
	mux          *http.ServeMux
	coauthoring  *ws.Handler
	cacheJanitor *cacheJanitor

	finishOnce sync.Once
	closeOnce  sync.Once
}

// New creates a document server. AssetDir may be empty for protocol-only testing.
func New(store Storage, opts Options) (*Server, error) {
	opts.normalize()
	if store == nil {
		return nil, errors.New("office: storage is required")
	}
	s := &Server{
		storage:  store,
		opts:     opts,
		sessions: session.NewManager(),
		mux:      http.NewServeMux(),
	}
	if opts.AssetDir != "" {
		s.cacheJanitor = newCacheJanitor(
			s.cacheDir(),
			opts.CacheTTL,
			opts.CacheMaxEntries,
			func(docKey string) { s.sessions.Delete(docKey) },
		)
		s.cacheJanitor.start()
	}
	s.buildRoutes()
	return s, nil
}

// Handler returns the HTTP handler for mounting on a host mux.
// Call Mount (e.g. demo routes) before serving; coauthoring fallback is registered on first use.
func (s *Server) Handler() http.Handler {
	s.finishOnce.Do(s.registerCoauthoringFallback)
	h := http.Handler(s.mux)
	if s.opts.Debug {
		h = debuglog.Middleware(s.opts.Logger, h)
	}
	return h
}

// Debug reports whether verbose logging is enabled.
func (s *Server) Debug() bool {
	return s.opts.Debug
}

// Mount registers an additional handler on the document server mux.
func (s *Server) Mount(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// BasePath returns the configured mount prefix (e.g. "/office").
func (s *Server) BasePath() string {
	return s.opts.BasePath
}

// DocumentServerURL returns the public URL prefix with trailing slash for Vue documentServerUrl.
func (s *Server) DocumentServerURL(publicOrigin string) string {
	publicOrigin = strings.TrimSuffix(publicOrigin, "/")
	base := strings.TrimSuffix(s.opts.BasePath, "/")
	if base == "" || base == "/" {
		return publicOrigin + "/"
	}
	return publicOrigin + base + "/"
}

// BuildEditorConfig returns ONLYOFFICE editor init JSON for the Vue component.
func (s *Server) BuildEditorConfig(ctx context.Context, req config.EditorRequest) (map[string]any, error) {
	if req.DocumentKey == "" {
		return nil, errors.New("office: document key is required")
	}
	s.sessions.UpsertDoc(session.Document{
		Key:         req.DocumentKey,
		Path:        req.StoragePath,
		URL:         req.DocumentURL,
		FileType:    req.FileType,
		CallbackURL: req.CallbackURL,
	})

	token := ""
	if len(s.opts.JWTSecret) > 0 {
		cfg := config.Build(req, "")
		t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(cfg))
		sig, err := t.SignedString(s.opts.JWTSecret)
		if err != nil {
			return nil, fmt.Errorf("office: sign config: %w", err)
		}
		token = sig
	}
	return config.Build(req, token), nil
}

// Close releases in-memory session state and stops background workers.
func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.coauthoring != nil {
			s.coauthoring.Stop()
		}
		if s.cacheJanitor != nil {
			s.cacheJanitor.stop()
		}
	})
	return err
}

func (s *Server) buildRoutes() {
	prefix := s.opts.BasePath

	s.mux.HandleFunc(joinURLPath(prefix, "health"), s.handleHealth)
	s.mux.HandleFunc(joinURLPath(prefix, "healthz"), s.handleHealth)
	s.mux.HandleFunc(joinURLPath(prefix, "healthcheck"), s.handleHealthCheck)
	s.mux.HandleFunc(joinURLPath(prefix, "info/info.json"), s.handleInfoJSON)
	s.mux.HandleFunc(joinURLPath(prefix, "plugins.json"), s.handlePluginsJSON)

	if s.opts.AssetDir != "" {
		webApps := static.Dir(s.opts.AssetDir, "web-apps")
		sdkjs := static.Dir(s.opts.AssetDir, "sdkjs")
		fonts := static.Dir(s.opts.AssetDir, "fonts")
		if webApps != nil {
			s.mux.Handle(joinURLPath(prefix, "web-apps/"), http.StripPrefix(joinURLPath(prefix, "web-apps"), webApps))
		}
		if sdkjs != nil {
			s.mux.Handle(joinURLPath(prefix, "sdkjs/"), http.StripPrefix(joinURLPath(prefix, "sdkjs"), sdkjs))
		}
		if fonts != nil {
			s.mux.Handle(joinURLPath(prefix, "fonts/"), http.StripPrefix(joinURLPath(prefix, "fonts"), fonts))
		}
		s.mux.HandleFunc(joinURLPath(prefix, "document_editor_service_worker.js"), s.handleServiceWorker)
		if mirrorAssetsAtRoot(prefix) {
			if webApps != nil {
				s.mux.Handle("/web-apps/", http.StripPrefix("/web-apps/", webApps))
			}
			if sdkjs != nil {
				s.mux.Handle("/sdkjs/", http.StripPrefix("/sdkjs/", sdkjs))
			}
			if fonts != nil {
				s.mux.Handle("/fonts/", http.StripPrefix("/fonts/", fonts))
			}
			s.mux.HandleFunc("/document_editor_service_worker.js", s.handleServiceWorker)
			s.mux.HandleFunc("/plugins.json", s.handlePluginsJSON)
		}
	}

	cacheDir := s.cacheDir()
	_ = os.MkdirAll(cacheDir, 0o755)
	cachePrefix := joinURLPath(prefix, "cache/files")
	s.mux.Handle(cachePrefix+"/", http.StripPrefix(cachePrefix, http.FileServer(http.Dir(cacheDir))))

	s.mux.HandleFunc("/downloadfile/", s.handleDownloadFile)
	if base := strings.Trim(strings.TrimSpace(prefix), "/"); base != "" && !mirrorAssetsAtRoot(prefix) {
		s.mux.HandleFunc("/"+base+"/downloadfile/", s.handleDownloadFile)
	}
}

func (s *Server) registerCoauthoringFallback() {
	prefix := s.opts.BasePath
	var opener *ws.Opener
	if conv, err := convert.New(convert.Options{
		AssetDir: s.opts.AssetDir,
		Limit:    s.opts.ConvertLimit,
	}); err == nil {
		opener = &ws.Opener{
			Converter: conv,
			CacheDir:  s.cacheDir(),
			Logger:    s.opts.Logger,
		}
	} else if s.opts.Debug {
		s.opts.Logger.Debug("coauthoring converter unavailable", "err", err)
	}
	co := ws.NewWithOptions(ws.HandlerOptions{
		Version:      s.opts.ProtocolVersion,
		BasePath:     s.opts.BasePath,
		Logger:       s.opts.Logger,
		Debug:        s.opts.Debug,
		PollHold:     s.opts.PollHold,
		PublicOrigin: s.opts.PublicOrigin,
		Opener:       opener,
		CacheDir:     s.cacheDir(),
		Saver:        s,
	})
	s.coauthoring = co
	docPattern := joinURLPath(prefix, "doc") + "/"
	s.mux.Handle(docPattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := stripMountPath(prefix, r.URL.Path)
		co.ServePath(w, r, rel)
	}))

	mount := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	catch := "/"
	if mount != "" {
		catch = mount + "/"
	}
	s.mux.Handle(catch, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := stripMountPath(prefix, r.URL.Path)
		if _, ok := ws.Match(strings.Trim(rel, "/")); ok {
			co.ServePath(w, r, rel)
			return
		}
		http.NotFound(w, r)
	}))
}

func (s *Server) cacheDir() string {
	if s.opts.AssetDir == "" {
		return os.TempDir()
	}
	return s.opts.AssetDir + string(os.PathSeparator) + "cache"
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cacheDir := s.cacheDir()
	_, _ = w.Write([]byte(`{"status":"ok","version":` + strconvQuote(s.opts.ProtocolVersion) +
		`,"sessions":` + fmt.Sprintf("%d", s.sessions.Len()) +
		`,"cacheDirs":` + fmt.Sprintf("%d", cacheDirCount(cacheDir)) +
		`,"cacheBytes":` + fmt.Sprintf("%d", cacheDirSize(cacheDir)) + `}`))
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("true"))
}

func (s *Server) handleInfoJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"version":` + strconvQuote(s.opts.ProtocolVersion) + `}`))
}

func (s *Server) handlePluginsJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write([]byte("[]"))
}

func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := s.opts.AssetDir + string(os.PathSeparator) + "sdkjs" + string(os.PathSeparator) +
		"common" + string(os.PathSeparator) + "serviceworker" + string(os.PathSeparator) +
		"document_editor_service_worker.js"
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// mirrorAssetsAtRoot reports whether editor assets should also be served from /sdkjs and /web-apps.
// The Euro-Office editor iframe resolves some script paths from the site root.
func mirrorAssetsAtRoot(basePath string) bool {
	basePath = strings.TrimSuffix(strings.TrimSpace(basePath), "/")
	return basePath != "" && basePath != "/"
}

func stripMountPath(prefix, urlPath string) string {
	mount := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	if mount == "" || mount == "/" {
		return strings.TrimPrefix(urlPath, "/")
	}
	return strings.TrimPrefix(strings.TrimPrefix(urlPath, mount), "/")
}
