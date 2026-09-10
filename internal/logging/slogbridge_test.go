package logging

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewSlogProducesLogger(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"text", Options{Debug: true, JSON: false}},
		{"json", Options{Debug: false, JSON: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := NewSlog(tc.opts)
			if log == nil {
				t.Fatal("logger is nil")
			}
			if !log.Enabled(context.Background(), slog.LevelInfo) {
				t.Fatal("info should be enabled")
			}
		})
	}
}

func TestBridgeHandlerRespectsDebugLevel(t *testing.T) {
	log := NewSlog(Options{Debug: false, JSON: false})
	if log.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug should be disabled when debug=false")
	}
	if !log.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info should be enabled")
	}
}

func TestJSONEnabled(t *testing.T) {
	t.Setenv("OFFICE_LOG_JSON", "1")
	if !JSONEnabled() {
		t.Fatal("expected JSON enabled")
	}
	t.Setenv("OFFICE_LOG_JSON", "")
	if JSONEnabled() {
		t.Fatal("expected JSON disabled")
	}
}
