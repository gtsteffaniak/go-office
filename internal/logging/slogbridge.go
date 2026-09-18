package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Options configures go-office logging.
type Options struct {
	Debug bool
	JSON  bool
}

// NewSlog returns a slog.Logger that writes to stderr (unbuffered via line breaks).
// We use the stdlib handlers directly so logs appear in CI/Docker captures reliably.
func NewSlog(opts Options) *slog.Logger {
	minLvl := slog.LevelInfo
	if opts.Debug {
		minLvl = slog.LevelDebug
	}
	handlerOpts := &slog.HandlerOptions{Level: minLvl}
	if opts.JSON {
		return slog.New(slog.NewJSONHandler(os.Stderr, handlerOpts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, handlerOpts))
}

// JSONEnabled reports whether OFFICE_LOG_JSON requests JSON log output.
func JSONEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("OFFICE_LOG_JSON")))
	return v == "1" || v == "true" || v == "yes"
}
