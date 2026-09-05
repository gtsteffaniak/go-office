package debuglog

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/quantumx-apps/go-office/internal/logging"
	"github.com/quantumx-apps/go-office/internal/ws"
)

// Enabled reports whether debug logging is on via -debug flag or environment.
func Enabled(flag bool) bool {
	return flag || EnvEnabled()
}

// EnvEnabled reports whether OFFICE_DEBUG_LOGGING (or legacy OFFICE_DEBUG) is set.
func EnvEnabled() bool {
	for _, key := range []string{"OFFICE_DEBUG_LOGGING", "OFFICE_DEBUG"} {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if v == "1" || v == "true" || v == "yes" {
			return true
		}
	}
	return false
}

// NewLogger returns a stderr logger backed by go-logger when available.
func NewLogger(debug bool) *slog.Logger {
	return logging.NewSlog(logging.Options{
		Debug: debug,
		JSON:  logging.JSONEnabled(),
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Middleware logs each HTTP request when logger is at debug level.
func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		if ws.IsCoauthoringPollingCheck(r) {
			return
		}
		logger.Debug("http",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", rw.status,
			"bytes", rw.bytes,
			"duration", time.Since(start).String(),
		)
		if rw.status >= 400 && !isExpectedHTTPError(r, rw.status) {
			logger.Warn("http error response",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
			)
		}
	})
}

// isExpectedHTTPError reports benign client probes that should not emit WARN lines.
func isExpectedHTTPError(r *http.Request, status int) bool {
	path := r.URL.Path
	switch {
	case status == http.StatusNotImplemented && strings.Contains(path, "/c/") &&
		(strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || r.URL.Query().Get("transport") == "websocket"):
		return true
	case status == http.StatusNotFound:
		switch {
		case path == "/favicon.ico":
			return true
		case path == "/themes.json":
			return true
		case strings.HasPrefix(path, "/dictionaries/"):
			return true
		case strings.HasPrefix(path, "/.well-known/"):
			return true
		}
	}
	return false
}
