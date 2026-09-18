package office

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quantumx-apps/go-office/internal/changes"
	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/session"
	"github.com/quantumx-apps/go-office/pkg/callback"
)

// maxPersistCoalesceRounds bounds mid-flush re-convert loops when change blobs arrive
// during x2t; prevents unbounded work if the journal keeps growing.
const maxPersistCoalesceRounds = 8

// HandleCallback processes ONLYOFFICE editor callback POSTs (status 2/6 → persist).
func (s *Server) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := callback.ReadRequest(r, s.opts.JWTSecret)
	if err != nil {
		callback.WriteError(w, 1)
		return
	}
	payload := body

	if s.opts.Debug && s.opts.Logger != nil {
		s.opts.Logger.Debug("office callback", "key", payload.Key, "status", payload.Status, "url", payload.URL)
	}

	if !payload.ShouldPersist() {
		callback.WriteOK(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	if err := s.persistFromCallback(ctx, payload); err != nil {
		if s.opts.Logger != nil {
			s.opts.Logger.Error("callback save failed", "key", payload.Key, "err", err)
		}
		callback.WriteError(w, 1)
		return
	}
	callback.WriteOK(w)
}

// PersistOutcome reports what persistDocument did.
type PersistOutcome struct {
	// AckBlobs is how many journal blobs were converted and acknowledged.
	AckBlobs int
	// RolledBack reports the target format was unwritable and OOXML bridge bytes were
	// persisted at the original path (assemblyFormatAsOrigin).
	RolledBack bool
	// Bridge names the OOXML format persisted when RolledBack is true.
	Bridge convert.SaveBridge
	// Bytes is the size of the persisted file.
	Bytes int64
	// Snapshot identifies the exact journal entries converted, for transactional
	// acknowledgement. Zero when nothing was converted.
	Snapshot changes.Snapshot
}

// PersistDocument converts Editor.bin in cache and writes to Storage.
func (s *Server) PersistDocument(ctx context.Context, docKey string) error {
	out, err := s.persistDocument(ctx, docKey, true)
	if err != nil {
		return err
	}
	return s.finalizePersist(docKey, out.AckBlobs)
}

func (s *Server) persistDocument(ctx context.Context, docKey string, ackOnSuccess bool) (PersistOutcome, error) {
	doc, ok := s.sessions.Lookup(docKey)
	if !ok {
		return PersistOutcome{}, fmt.Errorf("office: unknown document key %q", docKey)
	}
	callbackOnly := doc.Path == "" && strings.TrimSpace(doc.CallbackURL) != ""
	if doc.Path == "" && !callbackOnly {
		return PersistOutcome{}, fmt.Errorf("office: no storage path or callback URL for key %q", docKey)
	}
	ext := strings.TrimPrefix(strings.ToLower(doc.FileType), ".")
	if ext == "" {
		return PersistOutcome{}, fmt.Errorf("office: missing file type for key %q", docKey)
	}
	if convert.IsBrowserEditorFormat(ext) {
		return PersistOutcome{}, fmt.Errorf("office: save not supported for %s format", ext)
	}

	cacheDir := filepath.Join(s.cacheDir(), docKey)
	if !hasPendingChanges(cacheDir) {
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("persist skipped; no pending changes", "key", docKey)
		}
		return PersistOutcome{}, nil
	}

	conv, err := s.converter()
	if err != nil {
		return PersistOutcome{}, err
	}

	outPath := filepath.Join(cacheDir, "saved."+ext)
	pending := hasPendingChanges(cacheDir)
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("persist convert", "key", docKey, "path", doc.Path, "ext", ext, "pendingChanges", pending, "callbackOnly", callbackOnly)
	}
	var ackBlobs int
	var rolledBack bool
	var rollbackBridge convert.SaveBridge
	var lastSnapshot changes.Snapshot
	var outcome PersistOutcome
	for round := 0; ; round++ {
		if round >= maxPersistCoalesceRounds {
			return PersistOutcome{}, fmt.Errorf("office: coalesce round limit exceeded for key %q", docKey)
		}
		res, convErr := s.convertDocument(ctx, conv, cacheDir, outPath, ext)
		if convErr != nil {
			return PersistOutcome{}, convErr
		}
		ackBlobs = res.AckBlobs
		lastSnapshot = res.Snapshot
		if res.RolledBack {
			rolledBack = true
			rollbackBridge = res.Bridge
		}
		// converted is the number of journal entries the converter actually consumed. It is
		// the only sound measure of progress: the raw journal length can grow without bound
		// while a busy editor appends mid-flush, so comparing totals reports false "no
		// progress" (or hides real lack of progress) depending on timing.
		converted := res.Snapshot.BlobCount
		if converted == 0 {
			converted = ackBlobs
		}
		if converted == 0 {
			break
		}
		pendingAfterConvert, countErr := changes.Count(cacheDir)
		if countErr != nil {
			return PersistOutcome{}, countErr
		}
		if pendingAfterConvert <= converted {
			break
		}
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("persist coalescing mid-flush changes",
				"key", docKey,
				"converted", converted,
				"pending", pendingAfterConvert,
				"round", round+1,
			)
		}
		// New blobs arrived during x2t. Rebuild Editor.bin from the partial output and
		// acknowledge exactly the converted snapshot before applying the remainder.
		if refreshErr := s.refreshEditorBinFromSaved(ctx, cacheDir, outPath); refreshErr != nil {
			return PersistOutcome{}, refreshErr
		}
		removed, ackErr := changes.AcknowledgeSnapshot(cacheDir, res.Snapshot)
		if ackErr != nil {
			return PersistOutcome{}, ackErr
		}
		changes.RemoveSnapshot(cacheDir)
		if removed == 0 {
			// The journal no longer starts with the converted snapshot, so nothing was
			// removed and another round cannot make progress on the same snapshot.
			return PersistOutcome{}, fmt.Errorf(
				"office: coalesce made no journal progress for key %q (converted=%d pending=%d)",
				docKey, converted, pendingAfterConvert)
		}
	}
	if rolledBack && s.opts.Logger != nil {
		s.opts.Logger.Warn("saved with assembly rollback (OOXML bytes at original path)",
			"key", docKey,
			"ext", ext,
			"bridge", string(rollbackBridge),
			"path", doc.Path,
		)
	}

	f, err := os.Open(outPath)
	if err != nil {
		return PersistOutcome{}, err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return PersistOutcome{}, err
	}
	raw = convert.NormalizePersistedOutput(ext, raw)
	outcome = PersistOutcome{
		AckBlobs:   ackBlobs,
		RolledBack: rolledBack,
		Bridge:     rollbackBridge,
		Bytes:      int64(len(raw)),
		Snapshot:   lastSnapshot,
	}
	xlsxInfo, _ := os.Stat(filepath.Join(cacheDir, "changes-applied.xlsx"))
	xlsxBytes := int64(0)
	if xlsxInfo != nil {
		xlsxBytes = xlsxInfo.Size()
	}
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("persist output",
			"key", docKey,
			"savedBytes", len(raw),
			"xlsxBytes", xlsxBytes,
			"textPreview", persistTextPreview(ext, raw),
		)
	}

	if !callbackOnly {
		if err := s.storage.Save(ctx, doc.Path, bytes.NewReader(raw)); err != nil {
			return PersistOutcome{}, err
		}
		s.sessions.UpsertDoc(session.Document{Key: docKey, Path: doc.Path, FileType: ext, UpdatedAt: time.Now().UTC()})
		if s.opts.Logger != nil {
			s.opts.Logger.Info("document saved", "key", docKey, "path", doc.Path, "bytes", len(raw))
		}
	} else if s.opts.Logger != nil {
		s.opts.Logger.Info("document converted for callback", "key", docKey, "bytes", len(raw))
	}
	if ackBlobs > 0 {
		if err := s.refreshEditorBinFromSaved(ctx, cacheDir, outPath); err != nil {
			return PersistOutcome{}, err
		}
	}
	if ackOnSuccess {
		if err := s.finalizePersistOutcome(docKey, outcome); err != nil {
			return outcome, err
		}
		return outcome, nil
	}
	return outcome, nil
}

func (s *Server) finalizePersist(docKey string, ackBlobs int) error {
	return s.finalizePersistOutcome(docKey, PersistOutcome{AckBlobs: ackBlobs})
}

// finalizePersistOutcome acknowledges converted blobs and drops the transient snapshot.
// When the outcome carries the converted snapshot, acknowledgement is verified against the
// journal so blobs that were never applied are not silently discarded.
func (s *Server) finalizePersistOutcome(docKey string, outcome PersistOutcome) error {
	cacheDir := filepath.Join(s.cacheDir(), docKey)
	switch {
	case outcome.Snapshot.BlobCount > 0:
		removed, err := changes.AcknowledgeSnapshot(cacheDir, outcome.Snapshot)
		if err != nil {
			return err
		}
		if removed == 0 && outcome.AckBlobs > 0 && s.opts.Logger != nil {
			s.opts.Logger.Warn("save acknowledgement skipped; journal changed since conversion",
				"key", docKey, "converted", outcome.AckBlobs)
		}
	case outcome.AckBlobs > 0:
		if err := changes.Acknowledge(cacheDir, outcome.AckBlobs); err != nil {
			return err
		}
	default:
		if !hasPendingChanges(cacheDir) {
			_ = changes.Clear(cacheDir)
		}
	}
	changes.RemoveSnapshot(cacheDir)
	s.afterPersistCacheUpdate(cacheDir)
	return nil
}

func (s *Server) persistFromCallback(ctx context.Context, payload callback.Payload) error {
	doc, ok := s.sessions.Get(payload.Key)
	if !ok || doc.Path == "" {
		return fmt.Errorf("office: no storage mapping for key %q", payload.Key)
	}

	if payload.URL != "" {
		return s.downloadAndSave(ctx, payload.URL, doc.Path, doc.FileType)
	}
	cacheDir := filepath.Join(s.cacheDir(), payload.Key)
	if !hasPendingChanges(cacheDir) {
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("callback persist skipped; no pending changes", "key", payload.Key, "status", payload.Status)
		}
		return nil
	}
	return s.PersistDocument(ctx, payload.Key)
}

func (s *Server) downloadAndSave(ctx context.Context, rawURL, storagePath, fileType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", rawURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
	if err != nil {
		return err
	}
	body = convert.NormalizePersistedOutput(fileType, body)
	return s.storage.Save(ctx, storagePath, bytes.NewReader(body))
}

// NotifyCallback POSTs a save notification to the integrator callback URL.
func (s *Server) NotifyCallback(ctx context.Context, docKey, callbackURL, downloadURL string, force bool, doc session.Document) error {
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL == "" {
		return nil
	}
	status := callback.StatusMustSave
	forceSaveType := 0
	if force {
		status = callback.StatusForceSaved
		forceSaveType = 1
	}
	ext := strings.TrimPrefix(strings.ToLower(doc.FileType), ".")
	users := []string{}
	if doc.UserID != "" {
		users = []string{doc.UserID}
	}
	body, err := json.Marshal(callback.Payload{
		Key:           docKey,
		Status:        status,
		URL:           downloadURL,
		FileType:      ext,
		Users:         users,
		ForceSaveType: forceSaveType,
	})
	if err != nil {
		return err
	}
	postBody := body
	if len(s.opts.JWTSecret) > 0 {
		token, signErr := callback.SignBody(s.opts.JWTSecret, body)
		if signErr != nil {
			return signErr
		}
		wrapped, wrapErr := json.Marshal(map[string]string{"token": token})
		if wrapErr != nil {
			return wrapErr
		}
		postBody = wrapped
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(postBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("callback POST %s: %s", callbackURL, resp.Status)
	}
	return callback.ParseCallbackResponse(respBody)
}

// CacheFileURL builds a public URL for a file under cache/files/{key}/.
func (s *Server) CacheFileURL(origin, docKey, name string) string {
	origin = strings.TrimSuffix(strings.TrimSpace(origin), "/")
	if origin == "" {
		origin = strings.TrimSuffix(strings.TrimSpace(s.opts.PublicOrigin), "/")
	}
	base := strings.TrimSuffix(s.opts.BasePath, "/")
	if base == "" || base == "/" {
		return origin + "/cache/files/" + docKey + "/" + name
	}
	return origin + base + "/cache/files/" + docKey + "/" + name
}

// Sessions exposes the document session manager for integrators that wire coauthoring saves.
// Prefer FlushDocument and PersistDocument when possible.
func (s *Server) Sessions() *session.Manager {
	return s.sessions
}

// Storage returns the configured host storage backend.
func (s *Server) Storage() Storage {
	return s.storage
}

func headPreview(raw []byte) string {
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 2 {
		lines = lines[:2]
	}
	out := strings.Join(lines, "\n")
	if len(out) > 180 {
		return out[:180]
	}
	return out
}

func firstCSVDataCell(raw []byte) string {
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return ""
	}
	line := lines[1]
	if i := strings.IndexByte(line, ','); i >= 0 {
		return line[:i]
	}
	return line
}

func persistTextPreview(ext string, raw []byte) string {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "csv", "tsv", "scsv":
		cell := firstCSVDataCell(raw)
		prev := headPreview(raw)
		if cell != "" {
			return "cell=" + cell + " " + prev
		}
		return prev
	case "docx", "doc", "odt":
		text := convert.OOXMLPlainText(raw)
		if text == "" {
			return headPreview(raw)
		}
		if len(text) > 180 {
			return text[:180]
		}
		return text
	case "rtf":
		text := convert.RTFPlainText(raw)
		if text == "" {
			return headPreview(raw)
		}
		if len(text) > 180 {
			return text[:180]
		}
		return text
	default:
		return headPreview(raw)
	}
}
