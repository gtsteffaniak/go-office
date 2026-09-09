package office

import (
	"context"
	"fmt"
	"strings"

	"github.com/quantumx-apps/go-office/internal/changes"
	"github.com/quantumx-apps/go-office/internal/convert"
)

// FlushDocument implements ws.DocumentSaver for coauthoring save completion.
func (s *Server) FlushDocument(ctx context.Context, docKey, origin string, force bool) error {
	if err := s.PersistDocument(ctx, docKey); err != nil {
		return err
	}
	doc, ok := s.sessions.Lookup(docKey)
	if !ok || strings.TrimSpace(doc.CallbackURL) == "" {
		return nil
	}
	ext := strings.TrimPrefix(strings.ToLower(doc.FileType), ".")
	if ext == "" {
		return fmt.Errorf("office: missing file type for key %q", docKey)
	}
	downloadURL := s.CacheFileURL(origin, docKey, "saved."+ext)
	if err := s.NotifyCallback(ctx, docKey, doc.CallbackURL, downloadURL, force); err != nil {
		if s.opts.Logger != nil {
			s.opts.Logger.Warn("callback notify failed after persist", "key", docKey, "err", err)
		}
		// Storage was already updated in PersistDocument; don't fail the editor save for integrator notify errors.
	}
	return nil
}

func hasPendingChanges(cacheDir string) bool {
	return changes.HasPending(cacheDir)
}

func (s *Server) convertDocument(ctx context.Context, conv *convert.Converter, cacheDir, outPath, ext string) error {
	if hasPendingChanges(cacheDir) {
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("convert from changes", "cache", cacheDir, "ext", ext, "out", outPath)
		}
		if err := conv.SaveChanges(ctx, cacheDir, outPath, ext); err != nil {
			if s.opts.Logger != nil {
				s.opts.Logger.Warn("save changes conversion failed", "path", outPath, "err", err)
			}
			return err
		}
		return nil
	}
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("convert from Editor.bin only", "cache", cacheDir, "ext", ext)
	}
	return conv.FromEditorBin(ctx, cacheDir, outPath, ext)
}
