package ws

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const defaultPollHold = 20 * time.Second

// Handler serves ONLYOFFICE coauthoring endpoints at /doc/{key}/c/.
type Handler struct {
	Version  string
	Build    BuildInfo
	BasePath string
	Logger   *slog.Logger
	Debug    bool
	PollHold time.Duration
	Opener   *Opener
}

// HandlerOptions configures a coauthoring handler.
type HandlerOptions struct {
	Version  string
	BasePath string
	Logger   *slog.Logger
	Debug    bool
	Opener   *Opener
}

func New(version string, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		Version:  version,
		Build:    ParseBuild(version),
		Logger:   logger,
		PollHold: defaultPollHold,
	}
}

// NewWithOptions creates a coauthoring handler.
func NewWithOptions(opts HandlerOptions) *Handler {
	h := New(opts.Version, opts.Logger)
	h.Debug = opts.Debug
	h.BasePath = opts.BasePath
	h.Opener = opts.Opener
	return h
}

// Match reports whether path is a coauthoring route:
// /doc/{key}/c/... or /{version}/doc/{key}/c/...
func Match(path string) (key string, ok bool) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[0] == "doc" && parts[2] == "c" {
		return parts[1], true
	}
	if len(parts) >= 4 && parts[1] == "doc" && parts[3] == "c" {
		return parts[2], true
	}
	return "", false
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

	if r.URL.Query().Get("transport") == "polling" {
		h.servePolling(w, r, key)
		return
	}

	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "websocket coauthoring not implemented yet", http.StatusNotImplemented)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) pollHoldDuration() time.Duration {
	if h.PollHold > 0 {
		return h.PollHold
	}
	return defaultPollHold
}

func (h *Handler) servePolling(w http.ResponseWriter, r *http.Request, docKey string) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	sid := r.URL.Query().Get("sid")
	if sid == "" {
		sid = defaultSessionID
	}

	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if h.Debug {
			h.Logger.Debug("coauthoring polling post", "key", docKey, "sid", sid, "body", string(body))
		}
		sess := getSession(sid, docKey, h.Build, h.BasePath)
		for _, packet := range parsePostPackets(string(body)) {
			switch {
			case strings.HasPrefix(packet, "40"):
				sess.onConnect(connectAuthData(packet))
				if req, ok := parseAuthPacket(packet); ok {
					sess.startOpen(r, h.Opener, req)
				}
			case strings.HasPrefix(packet, "42"):
				if req, ok := parseAuthPacket(packet); ok {
					sess.onAuth(req)
					sess.startOpen(r, h.Opener, req)
				}
			}
		}
		_, _ = w.Write([]byte("ok"))
		return
	}

	if r.URL.Query().Get("sid") == "" {
		if h.Debug {
			h.Logger.Debug("coauthoring polling open", "key", docKey)
		}
		_, _ = w.Write([]byte(`0{"sid":"go-office","upgrades":[],"pingInterval":25000,"pingTimeout":20000}`))
		return
	}

	sess := getSession(sid, docKey, h.Build, h.BasePath)
	if packets := sess.waitForPackets(r.Context(), h.pollHoldDuration()); len(packets) > 0 {
		if h.Debug {
			h.Logger.Debug("coauthoring polling send", "key", docKey, "sid", sid, "packets", len(packets))
		}
		_, _ = w.Write([]byte(joinPackets(packets)))
		return
	}

	_, _ = w.Write([]byte("6"))
}
