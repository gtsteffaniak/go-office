package office

import (
	"context"
	"strings"
	"time"
)

const warmMaxAttempts = 4

func isWarmRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "exit status 80") ||
		strings.Contains(msg, "exit status 1") ||
		strings.Contains(msg, "resource temporarily unavailable")
}

// EnsureEditorBinWarm prepares Editor.bin like EnsureEditorBin but retries transient
// x2t failures that often occur when warm races an in-flight save convert.
func (s *Server) EnsureEditorBinWarm(ctx context.Context, docKey, sourcePath, ext string) error {
	var lastErr error
	for attempt := 0; attempt < warmMaxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*150) * time.Millisecond
			if s.opts.Logger != nil {
				s.opts.Logger.Debug("warm retry",
					"key", docKey,
					"attempt", attempt+1,
					"backoff", backoff,
					"err", lastErr,
				)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		lastErr = s.EnsureEditorBin(ctx, docKey, sourcePath, ext)
		if lastErr == nil {
			return nil
		}
		if !isWarmRetryable(lastErr) {
			return lastErr
		}
	}
	return lastErr
}
