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

// HandleCallback processes ONLYOFFICE editor callback POSTs (status 2/6 → persist).
func (s *Server) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := callback.ReadBodyWithSecret(r.Body, s.opts.JWTSecret)
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

// PersistDocument converts Editor.bin in cache and writes to Storage.
func (s *Server) PersistDocument(ctx context.Context, docKey string) error {
	doc, ok := s.sessions.Lookup(docKey)
	if !ok {
		return fmt.Errorf("office: unknown document key %q", docKey)
	}
	callbackOnly := doc.Path == "" && strings.TrimSpace(doc.CallbackURL) != ""
	if doc.Path == "" && !callbackOnly {
		return fmt.Errorf("office: no storage path or callback URL for key %q", docKey)
	}
	ext := strings.TrimPrefix(strings.ToLower(doc.FileType), ".")
	if ext == "" {
		return fmt.Errorf("office: missing file type for key %q", docKey)
	}
	if convert.IsBrowserEditorFormat(ext) {
		return fmt.Errorf("office: save not supported for %s format", ext)
	}

	cacheDir := filepath.Join(s.cacheDir(), docKey)
	if !hasPendingChanges(cacheDir) && !convert.EditorBinCached(cacheDir) {
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("persist skipped; no pending changes or Editor.bin", "key", docKey)
		}
		return nil
	}

	conv, err := s.converter()
	if err != nil {
		return err
	}

	outPath := filepath.Join(cacheDir, "saved."+ext)
	pending := hasPendingChanges(cacheDir)
	pendingCount := 0
	if pending {
		pendingCount, _ = changes.Count(cacheDir)
	}
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("persist convert", "key", docKey, "path", doc.Path, "ext", ext, "pendingChanges", pending, "pendingBlobs", pendingCount, "callbackOnly", callbackOnly)
	}
	if err = s.convertDocument(ctx, conv, cacheDir, outPath, ext); err != nil {
		return err
	}

	f, err := os.Open(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	raw = convert.NormalizePersistedOutput(ext, raw)
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
			return err
		}
		s.sessions.UpsertDoc(session.Document{Key: docKey, Path: doc.Path, FileType: ext, UpdatedAt: time.Now().UTC()})
		if s.opts.Logger != nil {
			s.opts.Logger.Info("document saved", "key", docKey, "path", doc.Path, "bytes", len(raw))
		}
	} else if s.opts.Logger != nil {
		s.opts.Logger.Info("document converted for callback", "key", docKey, "bytes", len(raw))
	}
	_ = os.Remove(filepath.Join(cacheDir, "source.sha256"))
	if pendingCount > 0 {
		if err := changes.Acknowledge(cacheDir, pendingCount); err != nil {
			return err
		}
	} else {
		_ = changes.Clear(cacheDir)
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
func (s *Server) NotifyCallback(ctx context.Context, docKey, callbackURL, downloadURL string, force bool) error {
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL == "" {
		return nil
	}
	status := callback.StatusMustSave
	if force {
		status = callback.StatusForceSaved
	}
	body, err := json.Marshal(callback.Payload{
		Key:    docKey,
		Status: status,
		URL:    downloadURL,
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
	if resp.StatusCode >= 300 {
		return fmt.Errorf("callback POST %s: %s", callbackURL, resp.Status)
	}
	return nil
}

// CacheFileURL builds a public URL for a file under cache/files/{key}/.
func (s *Server) CacheFileURL(origin, docKey, name string) string {
	origin = strings.TrimSuffix(origin, "/")
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
