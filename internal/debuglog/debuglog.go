package debuglog

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Enabled reports whether debug logging is on via flag or GO_OFFICE_DEBUG=1|true.
func Enabled(flag bool) bool {
	if flag {
		return true
	}
	v := strings.ToLower(strings.TrimSpace(os.Getenv("GO_OFFICE_DEBUG")))
	return v == "1" || v == "true" || v == "yes"
}

// NewLogger returns a stderr logger at debug level when enabled, otherwise info.
func NewLogger(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
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
		logger.Debug("http",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", rw.status,
			"bytes", rw.bytes,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
			"referer", r.Referer(),
			"ua", r.UserAgent(),
		)
		if rw.status >= 400 {
			logger.Warn("http error response",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
			)
		}
	})
}
