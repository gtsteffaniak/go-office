package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// Options configures go-office logging via gtsteffaniak/go-logger.
type Options struct {
	Debug bool
	JSON  bool
}

// NewSlog returns a slog.Logger backed by go-logger formatters.
func NewSlog(opts Options) *slog.Logger {
	levels := "INFO,WARNING,ERROR"
	if opts.Debug {
		levels = "INFO,DEBUG,WARNING,ERROR"
	}
	cfg := logger.JsonConfig{
		Levels:     levels,
		Structured: true,
		Json:       opts.JSON,
		NoColors:   opts.JSON,
		Output:     "stdout",
	}
	gl, err := logger.NewLogger(cfg)
	minLvl := slog.LevelInfo
	if opts.Debug {
		minLvl = slog.LevelDebug
	}
	if err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: minLvl}))
	}
	return slog.New(&bridgeHandler{log: gl, minLevel: minLvl})
}

type bridgeHandler struct {
	log      logger.Logger
	group    string
	attrs    []slog.Attr
	minLevel slog.Level
}

func (h *bridgeHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.log != nil && level >= h.minLevel
}

func (h *bridgeHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.log == nil {
		return nil
	}
	args := h.recordArgs(r)
	switch {
	case r.Level >= slog.LevelError:
		h.log.ErrorContext(ctx, r.Message, args...)
	case r.Level >= slog.LevelWarn:
		h.log.WarnContext(ctx, r.Message, args...)
	case r.Level >= slog.LevelDebug:
		h.log.DebugContext(ctx, r.Message, args...)
	default:
		h.log.InfoContext(ctx, r.Message, args...)
	}
	return nil
}

func (h *bridgeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *bridgeHandler) WithGroup(name string) slog.Handler {
	next := *h
	if h.group != "" {
		next.group = h.group + "." + name
	} else {
		next.group = name
	}
	return &next
}

func (h *bridgeHandler) recordArgs(r slog.Record) []any {
	var args []any
	for _, attr := range h.attrs {
		args = append(args, attr.Key, attr.Value.Any())
	}
	r.Attrs(func(attr slog.Attr) bool {
		key := attr.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		args = append(args, key, attr.Value.Any())
		return true
	})
	return args
}

// JSONEnabled reports whether OFFICE_LOG_JSON requests JSON log output.
func JSONEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("OFFICE_LOG_JSON")))
	return v == "1" || v == "true" || v == "yes"
}
