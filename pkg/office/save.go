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

	payload, err := callback.ReadBody(r.Body)
	if err != nil {
		callback.WriteError(w, 1)
		return
	}

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
	doc, ok := s.sessions.Get(docKey)
	if !ok {
		return fmt.Errorf("office: unknown document key %q", docKey)
	}
	if doc.Path == "" {
		return fmt.Errorf("office: no storage path for key %q", docKey)
	}
	ext := strings.TrimPrefix(strings.ToLower(doc.FileType), ".")
	if ext == "" {
		return fmt.Errorf("office: missing file type for key %q", docKey)
	}
	if convert.IsBrowserEditorFormat(ext) {
		return fmt.Errorf("office: save not supported for %s format", ext)
	}

	conv, err := convert.New(convert.Options{AssetDir: s.opts.AssetDir, Limit: s.opts.ConvertLimit})
	if err != nil {
		return err
	}

	cacheDir := filepath.Join(s.cacheDir(), docKey)
	outPath := filepath.Join(cacheDir, "saved."+ext)
	if err := s.convertDocument(ctx, conv, cacheDir, outPath, ext); err != nil {
		return err
	}

	f, err := os.Open(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := s.storage.Save(ctx, doc.Path, f); err != nil {
		return err
	}
	s.sessions.UpsertDoc(session.Document{Key: docKey, Path: doc.Path, FileType: ext, UpdatedAt: time.Now().UTC()})
	if s.opts.Logger != nil {
		s.opts.Logger.Info("document saved", "key", docKey, "path", doc.Path)
	}
	return nil
}

func (s *Server) persistFromCallback(ctx context.Context, payload callback.Payload) error {
	doc, ok := s.sessions.Get(payload.Key)
	if !ok || doc.Path == "" {
		return fmt.Errorf("office: no storage mapping for key %q", payload.Key)
	}

	if payload.URL != "" {
		return s.downloadAndSave(ctx, payload.URL, doc.Path)
	}
	return s.PersistDocument(ctx, payload.Key)
}

func (s *Server) downloadAndSave(ctx context.Context, rawURL, storagePath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", rawURL, resp.Status)
	}
	return s.storage.Save(ctx, storagePath, io.LimitReader(resp.Body, 128<<20))
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if len(s.opts.JWTSecret) > 0 {
		// Integrators often accept unsigned callbacks in dev; signing is Phase 1 follow-up.
	}
	resp, err := http.DefaultClient.Do(req)
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

// Sessions exposes the document session manager (for coauthoring save wiring).
func (s *Server) Sessions() *session.Manager {
	return s.sessions
}

// Storage returns the configured host storage backend.
func (s *Server) Storage() Storage {
	return s.storage
}
