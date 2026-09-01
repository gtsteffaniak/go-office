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
	mu       sync.Mutex
	timers   map[string]*time.Timer
	gen      map[string]uint64
	delay    time.Duration
	saver    DocumentSaver
	cacheDir string
	logger   *slog.Logger
}

func newSaveScheduler(cacheDir string, saver DocumentSaver, logger *slog.Logger, delayOverride *time.Duration) *saveScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	delay := defaultSaveDelay
	if delayOverride != nil {
		delay = *delayOverride
	}
	return &saveScheduler{
		timers:   make(map[string]*time.Timer),
		gen:      make(map[string]uint64),
		delay:    delay,
		saver:    saver,
		cacheDir: cacheDir,
		logger:   logger,
	}
}

func (s *saveScheduler) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, t := range s.timers {
		t.Stop()
		s.gen[key]++
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
	s.scheduleDone(docKey, origin, force, nil)
}

func (s *saveScheduler) scheduleDone(docKey, origin string, force bool, onDone func(error)) {
	if s == nil || s.saver == nil {
		if onDone != nil {
			onDone(nil)
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[docKey]; ok {
		t.Stop()
		delete(s.timers, docKey)
	}
	gen := s.gen[docKey] + 1
	s.gen[docKey] = gen
	delay := s.delay
	if force {
		delay = 0
	}
	s.timers[docKey] = time.AfterFunc(delay, func() {
		s.mu.Lock()
		delete(s.timers, docKey)
		if s.gen[docKey] != gen {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		s.logger.Debug("document flush start", "key", docKey, "force", force)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		err := s.saver.FlushDocument(ctx, docKey, origin, force)
		if err != nil {
			s.logger.Error("document flush failed", "key", docKey, "err", err)
		} else {
			s.logger.Debug("document flush ok", "key", docKey, "force", force)
			clearChanges(filepath.Join(s.cacheDir, docKey))
		}
		if onDone != nil {
			onDone(err)
		}
	})
}

func (h *Handler) handleSaveMessage(sess *session, msg map[string]any, docKey string, r *http.Request) bool {
	switch messageType(msg) {
	case "isSaveLock":
		if h.Logger != nil {
			h.Logger.Debug("isSaveLock", "key", docKey)
		}
		pkt, err := socketMessage(map[string]any{
			"type":     "saveLock",
			"saveLock": false,
		})
		if err == nil {
			sess.enqueue(pkt)
		}
		return true
	case "saveChanges":
		if h.Logger != nil {
			h.Logger.Info("saveChanges", "key", docKey, "blobs", len(parseChanges(msg)))
		}
		h.handleSaveChanges(sess, msg, docKey, r)
		return true
	case "getLock":
		h.handleGetLock(sess, msg, docKey)
		return true
	case "forceSaveStart":
		h.handleForceSaveStart(sess, docKey, r)
		return true
	case "getMessages":
		pkt, err := socketMessage(map[string]any{"type": "message", "messages": []any{}})
		if err == nil {
			sess.enqueue(pkt)
		}
		return true
	case "authChangesAck":
		return true
	default:
		return false
	}
}

// c_oAscServerCommandErrors from ONLYOFFICE commonDefines.
const (
	cmdNoError      = 0
	cmdNotModified  = 4
	forceSaveButton = 1 // c_oAscForceSaveTypes.Button
)

func (h *Handler) handleGetLock(sess *session, msg map[string]any, docKey string) {
	now := time.Now().UnixMilli()
	user := sess.participantID()
	locks := map[string]any{}
	for _, block := range lockBlocks(msg) {
		locks[lockKey(block)] = map[string]any{
			"time":  now,
			"user":  user,
			"block": block,
		}
	}
	pkt, err := socketMessage(map[string]any{
		"type":  "getLock",
		"locks": locks,
	})
	if err == nil {
		sess.enqueue(pkt)
	}
	if h.Logger != nil {
		h.Logger.Info("getLock", "key", docKey, "user", user, "locks", len(locks))
	}
}

func (h *Handler) handleForceSaveStart(sess *session, docKey string, r *http.Request) {
	now := time.Now().UnixMilli()
	pending := false
	if h.Scheduler != nil && h.Scheduler.cacheDir != "" {
		pending = hasPendingChanges(filepath.Join(h.Scheduler.cacheDir, docKey))
	}

	if h.Logger != nil {
		h.Logger.Debug("forceSaveStart", "key", docKey, "pending", pending)
	}
	if !pending {
		pkt, err := socketMessage(map[string]any{
			"type": "forceSaveStart",
			"messages": map[string]any{
				"code":       cmdNotModified,
				"time":       nil,
				"inProgress": false,
			},
		})
		if err == nil {
			sess.enqueue(pkt)
		}
		return
	}

	start, err := socketMessage(map[string]any{
		"type": "forceSaveStart",
		"messages": map[string]any{
			"code":       cmdNoError,
			"time":       now,
			"inProgress": true,
		},
	})
	if err == nil {
		sess.enqueue(start)
	}

	h.Scheduler.scheduleDone(docKey, requestOrigin(r), true, func(flushErr error) {
		pkt, perr := socketMessage(map[string]any{
			"type": "forceSave",
			"messages": map[string]any{
				"type":    forceSaveButton,
				"time":    now,
				"success": flushErr == nil,
			},
		})
		if perr == nil {
			sess.enqueue(pkt)
		}
	})
}

func (h *Handler) handleSaveChanges(sess *session, msg map[string]any, docKey string, r *http.Request) {
	cacheDir := ""
	if h.Scheduler != nil {
		cacheDir = h.Scheduler.cacheDir
	}
	blobs := parseChanges(msg)
	startIndex := 0
	index := 0
	if cacheDir != "" {
		docCache := filepath.Join(cacheDir, docKey)
		var err error
		startIndex, err = maxChangeIndex(docCache)
		if err != nil && h.Logger != nil {
			h.Logger.Error("saveChanges index failed", "key", docKey, "err", err)
		}
		index, err = appendChanges(docCache, blobs)
		if err != nil && h.Logger != nil {
			h.Logger.Error("saveChanges write failed", "key", docKey, "err", err)
		} else if h.Logger != nil {
			first := ""
			if len(blobs) > 0 {
				preview := changeBlobPreview(blobs[:1])
				if len(preview) > 0 {
					first = preview[0]
				}
			}
			h.Logger.Debug("saveChanges queued",
				"key", docKey,
				"index", index,
				"blobs", len(blobs),
				"reSave", messageBool(msg, "reSave"),
				"endSaveChanges", messageBool(msg, "endSaveChanges"),
				"firstBlob", first,
			)
		}
	}

	now := time.Now().UnixMilli()
	unSaveIndex := -1
	if messageBool(msg, "startSaveChanges") && messageInt(msg, "deleteIndex", 1) == -1 {
		unSaveIndex = startIndex
	}
	pkt, err := socketMessage(map[string]any{
		"type":             "unSaveLock",
		"index":            unSaveIndex,
		"time":             now,
		"syncChangesIndex": index,
	})
	if err == nil {
		sess.enqueue(pkt)
	}

	force := messageBool(msg, "reSave")
	if h.Scheduler != nil {
		h.Scheduler.schedule(docKey, requestOrigin(r), force)
	}
}
