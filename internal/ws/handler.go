package ws

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const defaultPollHold = 2 * time.Second

// DocumentOpener opens or converts a document for coauthoring.
type DocumentOpener interface {
	Open(ctx context.Context, origin, basePath, docKey string, cmd openCmd) ([]string, error)
}

// Handler serves ONLYOFFICE coauthoring endpoints at /doc/{key}/c/.
type Handler struct {
	Version      string
	Build        BuildInfo
	BasePath     string
	Logger       *slog.Logger
	Debug        bool
	PollHold     time.Duration
	PublicOrigin string
	JWTSecret    []byte
	Opener       *Opener
	openHook     DocumentOpener
	Scheduler    *saveScheduler
	saver        DocumentSaver
}

// HandlerOptions configures a coauthoring handler.
type HandlerOptions struct {
	Version                string
	BasePath               string
	Logger                 *slog.Logger
	Debug                  bool
	PollHold               *time.Duration
	PublicOrigin           string
	JWTSecret              []byte
	Opener                 *Opener
	OpenHook               DocumentOpener
	CacheDir               string
	Saver                  DocumentSaver
	SaveDelay              *time.Duration
	ForceSaveFallbackDelay *time.Duration
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
	if opts.PollHold != nil {
		h.PollHold = *opts.PollHold
	}
	h.PublicOrigin = opts.PublicOrigin
	h.JWTSecret = opts.JWTSecret
	h.Opener = opts.Opener
	h.openHook = opts.OpenHook
	h.saver = opts.Saver
	if opts.Saver != nil && opts.CacheDir != "" {
		h.Scheduler = newSaveScheduler(opts.CacheDir, opts.Saver, opts.Logger, opts.SaveDelay, opts.ForceSaveFallbackDelay)
	}
	return h
}

func (h *Handler) registerDocumentSession(docKey string, req authRequest) {
	if h.saver == nil {
		return
	}
	reg, ok := h.saver.(DocumentSessionRegistrar)
	if !ok {
		return
	}
	fileType := ""
	documentURL := ""
	if req.Open != nil {
		fileType = req.Open.Format
		documentURL = req.Open.URL
	}
	userID := req.User.ID
	if userID == "" {
		userID = "user"
	}
	reg.RegisterDocumentSession(docKey, req.IntegratorCallbackURL(), fileType, documentURL, userID)
}

func (h *Handler) documentOpener() DocumentOpener {
	if h.openHook != nil {
		return h.openHook
	}
	return h.Opener
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

	if h.Debug && !IsCoauthoringPollingCheck(r) {
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
	return h.PollHold
}

func (h *Handler) servePolling(w http.ResponseWriter, r *http.Request, docKey string) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	sid := strings.TrimSpace(r.URL.Query().Get("sid"))

	if r.Method == http.MethodPost {
		if sid == "" {
			sid = newSessionID()
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 16<<20))
		if h.Debug {
			h.Logger.Debug("coauthoring message", "key", docKey, "sid", sid, "body", summarizeCoauthoringBody(string(body)))
		}
		sess := getSession(sid, docKey, h.Build, h.BasePath)
		for _, packet := range parsePostPackets(string(body)) {
			switch {
			case strings.HasPrefix(packet, "40"):
				authData := connectAuthData(packet)
				req, hasAuth := parseAuthPacket(packet)
				if hasAuth {
					if err := verifyAuthJWT(h.JWTSecret, req.Token, docKey); err != nil {
						if h.Logger != nil {
							h.Logger.Warn("coauthoring auth jwt rejected", "key", docKey, "err", err)
						}
						// Reject the connection rather than completing the handshake.
						// Previously this called onConnect and continued, which opened the
						// document WITHOUT JWT verification while still logging a warning:
						// the check looked enforced but was not. `close` with the
						// jwtError code (4006) tells sdkjs the session was refused.
						sess.enqueue(closePacket(closeCodeJWTError))
						continue
					}
				}
				var deferAuth bool
				if hasAuth {
					deferAuth = sess.needsDocumentOpen(req)
				}
				sess.onConnect(authData, deferAuth)
				if hasAuth {
					h.registerDocumentSession(docKey, req)
					if sess.needsDocumentOpen(req) {
						sess.startOpen(h.documentOpener(), req, CoauthoringOrigin(h.PublicOrigin, r))
					} else if sess.shouldLogReconnect(req) {
						if h.Logger != nil {
							h.Logger.WithGroup("coauthoring").Info("reconnect",
								"key", docKey,
								"keptOpen", true,
							)
						}
					}
				}
			case strings.HasPrefix(packet, "42"):
				if req, ok := parseAuthPacket(packet); ok {
					if err := verifyAuthJWT(h.JWTSecret, req.Token, docKey); err != nil {
						if h.Logger != nil {
							h.Logger.Warn("coauthoring auth jwt rejected", "key", docKey, "err", err)
						}
						// Same reasoning as the 40 case: refuse the session instead of
						// continuing to serve an unverified document.
						sess.enqueue(closePacket(closeCodeJWTError))
						continue
					}
					h.registerDocumentSession(docKey, req)
					if sess.needsDocumentOpen(req) {
						sess.startOpen(h.documentOpener(), req, CoauthoringOrigin(h.PublicOrigin, r))
					} else {
						sess.onAuth(req)
					}
					continue
				}
				if msg, ok := parseSocketMessage(packet); ok {
					h.handleSaveMessage(sess, msg, docKey, r)
				}
			}
		}
		_, _ = w.Write([]byte("ok"))
		return
	}

	if sid == "" {
		sid = newSessionID()
		_, _ = w.Write([]byte(`0{"sid":"` + sid + `","upgrades":[],"pingInterval":25000,"pingTimeout":20000}`))
		return
	}

	sess := getSession(sid, docKey, h.Build, h.BasePath)
	if packets := sess.waitForPackets(r.Context(), h.pollHoldDuration()); len(packets) > 0 {
		if h.Debug && h.Logger != nil {
			h.Logger.Debug("coauthoring send", "key", docKey, "sid", sid, "types", packetTypes(packets), "detail", summarizeOutboundPackets(packets))
		}
		_, _ = w.Write([]byte(joinPackets(packets)))
		return
	}

	_, _ = w.Write([]byte("6"))
}

func summarizeCoauthoringBody(body string) string {
	if strings.Contains(body, `"saveChanges"`) {
		return fmt.Sprintf("<saveChanges %d bytes omitted>", len(body))
	}
	if len(body) > 400 {
		return body[:400] + "...(truncated)"
	}
	return body
}
