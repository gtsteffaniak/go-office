package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/quantumx-apps/go-office/internal/license"
)

// Handler serves ONLYOFFICE coauthoring endpoints at /{version}/doc/{key}/c/.
// Phase 0: HTTP long-poll stub that returns license JSON; full Socket.IO in Phase 1.
type Handler struct {
	Version string
	Logger  *slog.Logger
	Debug   bool
}

func New(version string, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{Version: version, Logger: logger}
}

// NewWithOptions creates a coauthoring handler.
func NewWithOptions(version string, logger *slog.Logger, debug bool) *Handler {
	h := New(version, logger)
	h.Debug = debug
	return h
}

// Match reports whether path is a coauthoring route: /{version}/doc/{key}/c/...
func Match(path string) (key string, ok bool) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	// version / doc / key / c / ...
	if len(parts) < 4 {
		return "", false
	}
	if parts[1] != "doc" || parts[3] != "c" {
		return "", false
	}
	return parts[2], true
}

// ServeHTTP implements http.Handler using the request URL path.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServePath(w, r, r.URL.Path)
}

// ServePath handles coauthoring for a path relative to the document server mount.
func (h *Handler) ServePath(w http.ResponseWriter, r *http.Request, path string) {
	key, ok := Match(strings.Trim(path, "/"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	if h.Debug {
		h.Logger.Debug("coauthoring",
			"method", r.Method,
			"path", path,
			"key", key,
			"transport", r.URL.Query().Get("transport"),
			"query", r.URL.RawQuery,
		)
	}

	// Socket.IO polling handshake (EIO=4) — minimal response for Phase 0 debugging.
	if r.URL.Query().Get("transport") == "polling" {
		h.servePolling(w, r, key)
		return
	}

	// WebSocket upgrade will be implemented with a Socket.IO library in Phase 1.
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "websocket coauthoring not implemented yet", http.StatusNotImplemented)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) servePolling(w http.ResponseWriter, r *http.Request, key string) {
	_ = key
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")

	// Engine.IO open packet: "0" + JSON session id payload
	if r.URL.Query().Get("t") == "" {
		if h.Debug {
			h.Logger.Debug("coauthoring polling open", "key", key)
		}
		_, _ = w.Write([]byte(`0{"sid":"go-office","upgrades":["websocket"],"pingInterval":25000,"pingTimeout":20000}`))
		return
	}

	if h.Debug {
		h.Logger.Debug("coauthoring polling license", "key", key)
	}

	// License message as Socket.IO message packet "42" + JSON array
	lic := license.Permissive(h.Version)
	payload, err := json.Marshal([]any{"message", lic})
	if err != nil {
		http.Error(w, "encode license", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(append([]byte("42"), payload...))
}
