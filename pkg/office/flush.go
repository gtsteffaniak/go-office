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
	ackBlobs, err := s.persistDocument(ctx, docKey, false)
	if err != nil {
		return err
	}
	doc, ok := s.sessions.Lookup(docKey)
	if !ok {
		return nil
	}
	callbackURL := strings.TrimSpace(doc.CallbackURL)
	if callbackURL != "" && ackBlobs > 0 {
		ext := strings.TrimPrefix(strings.ToLower(doc.FileType), ".")
		if ext == "" {
			return fmt.Errorf("office: missing file type for key %q", docKey)
		}
		downloadURL := s.CacheFileURL(origin, docKey, "saved."+ext)
		if err := s.NotifyCallback(ctx, docKey, callbackURL, downloadURL, force, doc); err != nil {
			return err
		}
	}
	if err := s.finalizePersist(docKey, ackBlobs); err != nil {
		return err
	}
	return nil
}

func hasPendingChanges(cacheDir string) bool {
	return changes.HasPending(cacheDir)
}

func (s *Server) convertDocument(ctx context.Context, conv *convert.Converter, cacheDir, outPath, ext string) (int, error) {
	if hasPendingChanges(cacheDir) {
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("convert from changes", "cache", cacheDir, "ext", ext, "out", outPath)
		}
		ackBlobs, err := conv.SaveChanges(ctx, cacheDir, outPath, ext)
		if err != nil {
			if s.opts.Logger != nil {
				s.opts.Logger.Warn("save changes conversion failed", "path", outPath, "err", err)
			}
			return 0, err
		}
		return ackBlobs, nil
	}
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("convert from Editor.bin only", "cache", cacheDir, "ext", ext)
	}
	if err := conv.FromEditorBin(ctx, cacheDir, outPath, ext); err != nil {
		return 0, err
	}
	return 0, nil
}
