package ws

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

const defaultSaveDelay = 5 * time.Second

// DocumentSaver flushes coauthoring changes to host storage and notifies callbacks.
type DocumentSaver interface {
	FlushDocument(ctx context.Context, docKey, origin string, force bool) error
}

type saveScheduler struct {
	mu      sync.Mutex
	timers  map[string]*time.Timer
	delay   time.Duration
	saver   DocumentSaver
	cacheDir string
	logger  *slog.Logger
}

func newSaveScheduler(cacheDir string, saver DocumentSaver, logger *slog.Logger) *saveScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &saveScheduler{
		timers:   make(map[string]*time.Timer),
		delay:    defaultSaveDelay,
		saver:    saver,
		cacheDir: cacheDir,
		logger:   logger,
	}
}

func (s *saveScheduler) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = make(map[string]*time.Timer)
}

// Stop cancels pending save timers.
func (h *Handler) Stop() {
	if h != nil && h.Scheduler != nil {
		h.Scheduler.stop()
	}
}

func (s *saveScheduler) schedule(docKey, origin string, force bool) {
	if s == nil || s.saver == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[docKey]; ok {
		t.Stop()
		delete(s.timers, docKey)
	}
	delay := s.delay
	if force {
		delay = 0
	}
	s.timers[docKey] = time.AfterFunc(delay, func() {
		s.mu.Lock()
		delete(s.timers, docKey)
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := s.saver.FlushDocument(ctx, docKey, origin, force); err != nil {
			s.logger.Error("document flush failed", "key", docKey, "err", err)
			return
		}
		clearChanges(filepath.Join(s.cacheDir, docKey))
	})
}

func (h *Handler) handleSaveMessage(sess *session, msg map[string]any, docKey string, r *http.Request) bool {
	switch messageType(msg) {
	case "isSaveLock":
		pkt, err := socketMessage(map[string]any{
			"type":     "saveLock",
			"saveLock": false,
		})
		if err == nil {
			sess.enqueue(pkt)
		}
		return true
	case "saveChanges":
		h.handleSaveChanges(sess, msg, docKey, r)
		return true
	default:
		return false
	}
}

func (h *Handler) handleSaveChanges(sess *session, msg map[string]any, docKey string, r *http.Request) {
	cacheDir := ""
	if h.Scheduler != nil {
		cacheDir = h.Scheduler.cacheDir
	}
	index := 0
	if cacheDir != "" {
		var err error
		index, err = appendChanges(filepath.Join(cacheDir, docKey), parseChanges(msg))
		if err != nil && h.Logger != nil {
			h.Logger.Error("saveChanges write failed", "key", docKey, "err", err)
		}
	}

	now := time.Now().UnixMilli()
	pkt, err := socketMessage(map[string]any{
		"type":  "unSaveLock",
		"index": index,
		"time":  now,
	})
	if err == nil {
		sess.enqueue(pkt)
	}

	force := messageBool(msg, "reSave")
	if h.Scheduler != nil {
		h.Scheduler.schedule(docKey, requestOrigin(r), force)
	}
}
