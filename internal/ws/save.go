package ws

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
)

const defaultSaveDelay = 5 * time.Second

// defaultForceSaveFallbackDelay matches ONLYOFFICE services.CoAuthoring.server.savetimeoutdelay (5s).
// Force-save flush is triggered by endSaveChanges; the fallback timer only runs when the editor
// never sends endSaveChanges (stale pending changes or empty save).
const defaultForceSaveFallbackDelay = 5 * time.Second

// DocumentSaver flushes coauthoring changes to host storage and notifies callbacks.
type DocumentSaver interface {
	FlushDocument(ctx context.Context, docKey, origin string, force bool) error
}

// FlushDetail is the outcome of a flush for clients that report richer save status.
type FlushDetail struct {
	// RolledBack reports the target format was unwritable and OOXML bridge bytes were
	// persisted at the original path (assemblyFormatAsOrigin).
	RolledBack bool
	// Bridge names the OOXML format persisted when RolledBack is true.
	Bridge string
	// Bytes is the size of the persisted file.
	Bytes int64
}

// DetailedDocumentSaver is an optional DocumentSaver extension. When a saver implements it,
// the scheduler uses the returned detail to enrich the broadcast save result.
type DetailedDocumentSaver interface {
	FlushDocumentDetailed(ctx context.Context, docKey, origin string, force bool) (FlushDetail, error)
}

// DocumentSessionRegistrar records integrator session metadata from coauthoring auth.
type DocumentSessionRegistrar interface {
	RegisterDocumentSession(docKey, callbackURL, fileType, documentURL, userID string)
}

type flushJob struct {
	origin string
	force  bool
	onDone func(error)
}

type keyFlushCoordinator struct {
	mu       sync.Mutex
	running  bool
	queued   *flushJob
	afterRun []func(error)
}

type saveScheduler struct {
	mu             sync.Mutex
	timers         map[string]*time.Timer
	gen            map[string]uint64
	delay          time.Duration
	forceFallback  time.Duration
	saver          DocumentSaver
	cacheDir       string
	logger         *slog.Logger
	inflight       sync.WaitGroup
	forceTimers    map[string]*time.Timer
	forceOnDone    map[string]func(error)
	forceOrigin    map[string]string
	saveIntent     map[string]bool
	pendingEndSave map[string]bool
	flushCoords    map[string]*keyFlushCoordinator
	// outcomeSeq numbers broadcast save results per document key so clients can
	// discard out-of-order events.
	outcomeSeq map[string]uint64
}

func newSaveScheduler(cacheDir string, saver DocumentSaver, logger *slog.Logger, delayOverride, forceFallbackOverride *time.Duration) *saveScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	delay := defaultSaveDelay
	if delayOverride != nil {
		delay = *delayOverride
	}
	forceFallback := defaultForceSaveFallbackDelay
	if forceFallbackOverride != nil {
		forceFallback = *forceFallbackOverride
	}
	return &saveScheduler{
		timers:         make(map[string]*time.Timer),
		gen:            make(map[string]uint64),
		delay:          delay,
		forceFallback:  forceFallback,
		saver:          saver,
		cacheDir:       cacheDir,
		logger:         logger,
		forceTimers:    make(map[string]*time.Timer),
		forceOnDone:    make(map[string]func(error)),
		forceOrigin:    make(map[string]string),
		saveIntent:     make(map[string]bool),
		pendingEndSave: make(map[string]bool),
		flushCoords:    make(map[string]*keyFlushCoordinator),
		outcomeSeq:     make(map[string]uint64),
	}
}

// nextOutcomeSeq returns the next per-key save-result sequence number.
func (s *saveScheduler) nextOutcomeSeq(docKey string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outcomeSeq[docKey]++
	return s.outcomeSeq[docKey]
}

func (s *saveScheduler) stop() {
	s.mu.Lock()
	for key, t := range s.timers {
		t.Stop()
		s.gen[key]++
	}
	s.timers = make(map[string]*time.Timer)
	for key, t := range s.forceTimers {
		t.Stop()
		delete(s.forceTimers, key)
		delete(s.forceOnDone, key)
		delete(s.forceOrigin, key)
	}
	s.saveIntent = make(map[string]bool)
	s.pendingEndSave = make(map[string]bool)
	s.mu.Unlock()
}

func (s *saveScheduler) drain(ctx context.Context) error {
	if s == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		s.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// FlushDocument implements DocumentSaver with per-key flush serialization for opener flush-on-open.
func (s *saveScheduler) FlushDocument(ctx context.Context, docKey, origin string, force bool) error {
	if s == nil || s.saver == nil {
		return nil
	}
	done := make(chan error, 1)
	s.submitFlush(docKey, flushJob{origin: origin, force: force, onDone: func(err error) {
		done <- err
	}})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop cancels pending save timers, waits for in-flight flushes, and clears sessions.
func (h *Handler) Stop() {
	if h == nil {
		return
	}
	if h.Scheduler != nil {
		h.Scheduler.stop()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.Scheduler.drain(ctx); err != nil && h.Logger != nil {
			h.Logger.Debug("coauthoring save drain", "err", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := waitDocumentOpens(ctx); err != nil && h.Logger != nil {
		h.Logger.Debug("coauthoring open drain", "err", err)
	}
	ClearAllSessions()
}

func (s *saveScheduler) isFlushRunning(docKey string) bool {
	if s == nil {
		return false
	}
	coord := s.coord(docKey)
	coord.mu.Lock()
	running := coord.running
	coord.mu.Unlock()
	return running
}

func (s *saveScheduler) coord(docKey string) *keyFlushCoordinator {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.flushCoords[docKey]
	if !ok {
		c = &keyFlushCoordinator{}
		s.flushCoords[docKey] = c
	}
	return c
}

func (s *saveScheduler) markSaveIntent(docKey string) {
	s.mu.Lock()
	s.saveIntent[docKey] = true
	s.mu.Unlock()
}

func (s *saveScheduler) takeSaveIntent(docKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.saveIntent[docKey] {
		return false
	}
	delete(s.saveIntent, docKey)
	return true
}

func (s *saveScheduler) takePendingEndSave(docKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.pendingEndSave[docKey] {
		return false
	}
	delete(s.pendingEndSave, docKey)
	return true
}

// markPendingEndSave records that a non-final saveChanges batch is awaiting its
// endSaveChanges. forceSaveStart consults this to flush immediately rather than waiting
// out the force-save fallback timer.
func (s *saveScheduler) markPendingEndSave(docKey string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.pendingEndSave[docKey] = true
	s.mu.Unlock()
}

func (s *saveScheduler) isForceArmed(docKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.forceOnDone[docKey]
	return ok
}

func (s *saveScheduler) schedule(docKey, origin string, force bool) {
	s.scheduleDone(docKey, origin, force, nil)
}

func (s *saveScheduler) scheduleImmediate(docKey, origin string, force bool) {
	s.submitFlush(docKey, flushJob{origin: origin, force: force})
}

func (s *saveScheduler) armForceSave(docKey, origin string, onDone func(error)) {
	if s == nil || s.saver == nil {
		if onDone != nil {
			onDone(nil)
		}
		return
	}
	s.mu.Lock()
	if t, ok := s.forceTimers[docKey]; ok {
		t.Stop()
	}
	s.forceOnDone[docKey] = onDone
	s.forceOrigin[docKey] = origin
	s.forceTimers[docKey] = time.AfterFunc(s.forceFallback, func() {
		onDone, origin := s.takeForceSave(docKey)
		if onDone == nil {
			return
		}
		if !hasPendingChanges(filepath.Join(s.cacheDir, docKey)) {
			// Editor sent forceSaveStart with nothing to flush (no change blobs arrived).
			if s.logger != nil {
				s.logger.Debug("forceSave fallback: no pending changes", "key", docKey)
			}
			onDone(nil)
			return
		}
		// Fallback when endSaveChanges never arrives (stale pending changes).
		s.scheduleDone(docKey, origin, true, onDone)
	})
	s.mu.Unlock()
}

func (s *saveScheduler) takeForceSave(docKey string) (func(error), string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.forceTimers[docKey]; ok {
		t.Stop()
		delete(s.forceTimers, docKey)
	}
	onDone := s.forceOnDone[docKey]
	delete(s.forceOnDone, docKey)
	origin := s.forceOrigin[docKey]
	delete(s.forceOrigin, docKey)
	return onDone, origin
}

func (s *saveScheduler) completeForceSaveIfArmed(docKey, origin string) bool {
	onDone, armedOrigin := s.takeForceSave(docKey)
	if onDone == nil {
		return false
	}
	if armedOrigin != "" {
		origin = armedOrigin
	}
	s.scheduleDone(docKey, origin, true, onDone)
	return true
}

func (s *saveScheduler) submitFlush(docKey string, job flushJob) {
	if s == nil || s.saver == nil {
		if job.onDone != nil {
			job.onDone(nil)
		}
		return
	}
	coord := s.coord(docKey)
	coord.mu.Lock()
	if coord.running {
		coord.queued = s.mergeFlushJob(coord.queued, job)
		coord.mu.Unlock()
		return
	}
	coord.running = true
	coord.mu.Unlock()

	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		current := job
		for {
			err := s.runOneFlush(docKey, current.origin, current.force)
			if current.onDone != nil {
				current.onDone(err)
			}

			coord.mu.Lock()
			after := coord.afterRun
			coord.afterRun = nil
			if coord.queued == nil {
				coord.running = false
				coord.mu.Unlock()
				for _, fn := range after {
					fn(err)
				}
				return
			}
			current = *coord.queued
			coord.queued = nil
			coord.mu.Unlock()
			for _, fn := range after {
				fn(err)
			}
		}
	}()
}

// attachFlushOnDone chains a force-save completion callback onto the in-flight flush.
// It does not queue another x2t pass; the waiter runs when the current flush finishes.
func (s *saveScheduler) attachFlushOnDone(docKey string, onDone func(error)) bool {
	if s == nil || onDone == nil {
		return false
	}
	coord := s.coord(docKey)
	coord.mu.Lock()
	if !coord.running {
		coord.mu.Unlock()
		return false
	}
	coord.afterRun = append(coord.afterRun, onDone)
	coord.mu.Unlock()
	return true
}

func (s *saveScheduler) mergeFlushJob(existing *flushJob, job flushJob) *flushJob {
	if existing == nil {
		merged := job
		return &merged
	}
	if job.force {
		existing.force = true
	}
	if job.origin != "" {
		existing.origin = job.origin
	}
	if job.onDone != nil {
		if existing.onDone != nil {
			prev := existing.onDone
			existing.onDone = func(err error) {
				prev(err)
				job.onDone(err)
			}
		} else {
			existing.onDone = job.onDone
		}
	}
	return existing
}

func (s *saveScheduler) runOneFlush(docKey, origin string, force bool) error {
	docCache := filepath.Join(s.cacheDir, docKey)

	s.logger.Debug("document flush start", "key", docKey, "force", force)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var detail FlushDetail
	var err error
	if detailed, ok := s.saver.(DetailedDocumentSaver); ok {
		detail, err = detailed.FlushDocumentDetailed(ctx, docKey, origin, force)
	} else {
		err = s.saver.FlushDocument(ctx, docKey, origin, force)
	}

	if err != nil {
		s.logger.Error("document flush failed", "key", docKey, "err", err)
		if convert.NonRecoverableConvertError(err) {
			clearChanges(docCache)
		} else {
			notifyForceSaveResult(docKey, false)
		}
		// Tell clients the flush failed. Without this the editor keeps believing the save
		// succeeded, because unSaveLock already cleared its retry timer.
		broadcastSaveOutcome(docKey, SaveOutcome{
			Key:      docKey,
			Success:  false,
			Force:    force,
			Sequence: s.nextOutcomeSeq(docKey),
			Error:    err.Error(),
		})
		return err
	}

	s.logger.Debug("document flush ok", "key", docKey, "force", force, "rolledBack", detail.RolledBack)
	// finalizePersist already acknowledges converted blobs. Only clear an empty
	// journal; do not compare counts before/after flush — acknowledging shrinks the
	// journal and can look like "no progress" even when new blobs arrived mid-flush.
	if !hasPendingChanges(docCache) {
		clearChanges(docCache)
	}
	broadcastSaveOutcome(docKey, SaveOutcome{
		Key:        docKey,
		Success:    true,
		Force:      force,
		RolledBack: detail.RolledBack,
		Bridge:     detail.Bridge,
		Bytes:      detail.Bytes,
		Sequence:   s.nextOutcomeSeq(docKey),
	})
	return nil
}

func (s *saveScheduler) scheduleDone(docKey, origin string, force bool, onDone func(error)) {
	if s == nil || s.saver == nil {
		if onDone != nil {
			onDone(nil)
		}
		return
	}
	s.mu.Lock()
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
	job := flushJob{origin: origin, force: force, onDone: onDone}
	if delay == 0 {
		s.mu.Unlock()
		s.submitFlush(docKey, job)
		return
	}
	s.timers[docKey] = time.AfterFunc(delay, func() {
		s.mu.Lock()
		delete(s.timers, docKey)
		if s.gen[docKey] != gen {
			s.mu.Unlock()
			if onDone != nil {
				onDone(nil)
			}
			return
		}
		s.mu.Unlock()
		s.submitFlush(docKey, job)
	})
	s.mu.Unlock()
}

func (h *Handler) handleSaveMessage(sess *session, msg map[string]any, docKey string, r *http.Request) bool {
	switch messageType(msg) {
	case "isSaveLock":
		saveLocked := false
		if h.Scheduler != nil {
			saveLocked = h.Scheduler.isFlushRunning(docKey)
			if !saveLocked {
				h.Scheduler.markSaveIntent(docKey)
			}
		}
		if h.Logger != nil {
			h.Logger.Debug("isSaveLock", "key", docKey, "saveLock", saveLocked)
		}
		pkt, err := socketMessage(map[string]any{
			"type":     "saveLock",
			"saveLock": saveLocked,
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
	case "clientLog":
		h.logClientMessage(docKey, msg)
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
	flushRunning := false
	if h.Scheduler != nil && h.Scheduler.cacheDir != "" {
		pending = hasPendingChanges(filepath.Join(h.Scheduler.cacheDir, docKey))
		flushRunning = h.Scheduler.isFlushRunning(docKey)
	}

	if h.Logger != nil {
		h.Logger.Debug("forceSaveStart", "key", docKey, "pending", pending, "flushRunning", flushRunning)
	}

	start, err := socketMessage(map[string]any{
		"type": "forceSaveStart",
		"messages": map[string]any{
			"code":       cmdNoError,
			"time":       now,
			"inProgress": flushRunning,
		},
	})
	if err == nil {
		sess.enqueue(start)
	}

	if h.Scheduler == nil {
		return
	}

	origin := CoauthoringOrigin(h.PublicOrigin, r)
	onDone := func(flushErr error) {
		success := flushErr == nil
		if h.Logger != nil {
			if flushErr != nil {
				h.Logger.Info("forceSave result", "key", docKey, "success", false, "err", flushErr)
			} else {
				h.Logger.Info("forceSave result", "key", docKey, "success", true, "pending", pending)
			}
		}
		pkt, perr := socketMessage(map[string]any{
			"type": "forceSave",
			"messages": map[string]any{
				"type":    forceSaveButton,
				"time":    now,
				"success": success,
			},
		})
		if perr == nil {
			sess.enqueue(pkt)
		}
	}

	if h.Scheduler.takePendingEndSave(docKey) {
		h.Scheduler.scheduleDone(docKey, origin, true, onDone)
		return
	}

	if flushRunning {
		h.Scheduler.attachFlushOnDone(docKey, onDone)
		h.Scheduler.armForceSave(docKey, origin, nil)
		return
	}

	h.Scheduler.armForceSave(docKey, origin, onDone)
}

func clientLogSeverity(text, level string) string {
	if strings.Contains(text, "Write_ToBinary2") ||
		strings.Contains(text, "Uncaught TypeError") ||
		strings.Contains(text, "Uncaught Error") {
		return "error"
	}
	if level == "error" || level == "warn" {
		return "warn"
	}
	if strings.Contains(text, "changesError") &&
		strings.Contains(text, "_CheckCanNotAddChanges") &&
		!strings.Contains(text, "Uncaught") {
		// sdkjs load-path diagnostic only; open continues unless paired with Uncaught*.
		return "debug"
	}
	if strings.Contains(text, "Error") {
		return "warn"
	}
	return "debug"
}

func (h *Handler) logClientMessage(docKey string, msg map[string]any) {
	if h.Logger == nil {
		return
	}
	text, _ := msg["msg"].(string)
	if text == "" {
		return
	}
	level, _ := msg["level"].(string)
	switch clientLogSeverity(text, level) {
	case "error":
		h.Logger.Warn("editor clientLog", "key", docKey, "level", level, "severity", "error", "msg", text)
	case "warn":
		h.Logger.Warn("editor clientLog", "key", docKey, "level", level, "severity", "warn", "msg", text)
	default:
		h.Logger.Debug("editor clientLog", "key", docKey, "level", level, "severity", "debug", "msg", text)
	}
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
	origin := CoauthoringOrigin(h.PublicOrigin, r)
	endSave := messageBool(msg, "endSaveChanges")

	if endSave && h.Scheduler != nil {
		if h.Scheduler.completeForceSaveIfArmed(docKey, origin) {
			return
		}
		// isSaveLock marks intent for the editor save dance; do not defer autosave
		// endSaveChanges until forceSaveStart or saves never persist (only forceSaveStart
		// arms the waiter via completeForceSaveIfArmed above).
		h.Scheduler.takeSaveIntent(docKey)
		forceFlush := messageBool(msg, "reSave")
		h.Scheduler.scheduleImmediate(docKey, origin, forceFlush)
		return
	}

	if h.Scheduler == nil {
		return
	}
	if h.Scheduler.isForceArmed(docKey) {
		// A force save is armed and will flush (or its fallback timer will). Record that this
		// batch's endSaveChanges is still outstanding so forceSaveStart can flush immediately
		// instead of waiting out the fallback, then still schedule a debounced flush so the
		// batch is persisted even if the force save never completes. Previously this returned
		// without scheduling anything, so an interrupted force save deferred the autosave by
		// the full force-save fallback delay (5s) regardless of the configured save delay.
		h.Scheduler.markPendingEndSave(docKey)
		h.Scheduler.schedule(docKey, origin, force)
		return
	}
	h.Scheduler.schedule(docKey, origin, force)
}

func notifyForceSaveResult(docKey string, success bool) {
	now := time.Now().UnixMilli()
	forEachSession(docKey, func(s *session) {
		pkt, err := socketMessage(map[string]any{
			"type": "forceSave",
			"messages": map[string]any{
				"type":    forceSaveButton,
				"time":    now,
				"success": success,
			},
		})
		if err == nil {
			s.enqueue(pkt)
		}
	})
}
