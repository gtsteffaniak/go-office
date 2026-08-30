package office

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/quantumx-apps/go-office/config"
	"github.com/quantumx-apps/go-office/internal/ws"
	"github.com/quantumx-apps/go-office/session"
	"github.com/quantumx-apps/go-office/static"
)

// Server is an embedded ONLYOFFICE-compatible document server.
type Server struct {
	storage Storage
	opts    Options

	sessions *session.Manager
	mux      *http.ServeMux
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
	s.buildRoutes()
	return s, nil
}

// Handler returns the HTTP handler for mounting on a host mux.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// BasePath returns the configured mount prefix (e.g. "/api/office").
func (s *Server) BasePath() string {
	return s.opts.BasePath
}

// DocumentServerURL returns the public URL prefix with trailing slash for Vue documentServerUrl.
func (s *Server) DocumentServerURL(publicOrigin string) string {
	publicOrigin = strings.TrimSuffix(publicOrigin, "/")
	base := s.opts.BasePath
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return publicOrigin + base + "/"
}

// BuildEditorConfig returns ONLYOFFICE editor init JSON for the Vue component.
func (s *Server) BuildEditorConfig(ctx context.Context, req config.EditorRequest) (map[string]any, error) {
	if req.DocumentKey == "" {
		return nil, errors.New("office: document key is required")
	}
	s.sessions.Upsert(req.DocumentKey, req.DocumentURL)

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

// Close releases in-memory session state.
func (s *Server) Close() error {
	return nil
}

func (s *Server) buildRoutes() {
	prefix := s.opts.BasePath
	if prefix == "" {
		prefix = "/"
	}

	s.mux.HandleFunc(prefix+"/health", s.handleHealth)
	s.mux.HandleFunc(prefix+"/healthz", s.handleHealth)

	if s.opts.AssetDir != "" {
		if h := static.Dir(s.opts.AssetDir, "web-apps"); h != nil {
			s.mux.Handle(prefix+"/web-apps/", http.StripPrefix(prefix+"/web-apps/", h))
		}
		if h := static.Dir(s.opts.AssetDir, "sdkjs"); h != nil {
			s.mux.Handle(prefix+"/sdkjs/", http.StripPrefix(prefix+"/sdkjs/", h))
		}
	}

	cacheDir := s.cacheDir()
	_ = os.MkdirAll(cacheDir, 0o755)
	s.mux.Handle(prefix+"/cache/files/", http.StripPrefix(prefix+"/cache/files/", http.FileServer(http.Dir(cacheDir))))

	co := ws.New(s.opts.ProtocolVersion, s.opts.Logger)
	s.mux.Handle(prefix+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, prefix)
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
	_, _ = w.Write([]byte(`{"status":"ok","version":` + strconvQuote(s.opts.ProtocolVersion) + `}`))
}

func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
