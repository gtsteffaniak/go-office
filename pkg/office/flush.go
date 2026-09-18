package office

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quantumx-apps/go-office/internal/changes"
	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/ws"
)

// FlushDocument implements ws.DocumentSaver for coauthoring save completion.
func (s *Server) FlushDocument(ctx context.Context, docKey, origin string, force bool) error {
	_, err := s.FlushDocumentDetailed(ctx, docKey, origin, force)
	return err
}

// FlushDocumentDetailed is FlushDocument plus rollback/size detail, used to report save
// status to coauthoring clients (see ws.SaveOutcome).
func (s *Server) FlushDocumentDetailed(ctx context.Context, docKey, origin string, force bool) (ws.FlushDetail, error) {
	outcome, err := s.persistDocument(ctx, docKey, false)
	if err != nil {
		return ws.FlushDetail{}, err
	}
	doc, ok := s.sessions.Lookup(docKey)
	if !ok {
		return ws.FlushDetail{}, nil
	}
	callbackURL := strings.TrimSpace(doc.CallbackURL)
	// Autosave (force=false) must still notify when a prior pass already converted
	// blobs; only skip the integrator callback for redundant force-save with no work.
	if callbackURL != "" && !(force && outcome.AckBlobs == 0) {
		ext := strings.TrimPrefix(strings.ToLower(doc.FileType), ".")
		if ext == "" {
			return ws.FlushDetail{}, fmt.Errorf("office: missing file type for key %q", docKey)
		}
		downloadURL := s.CacheFileURL(origin, docKey, "saved."+ext)
		if err := s.NotifyCallback(ctx, docKey, callbackURL, downloadURL, force, doc); err != nil {
			return ws.FlushDetail{}, err
		}
	}
	if err := s.finalizePersistOutcome(docKey, outcome); err != nil {
		return ws.FlushDetail{}, err
	}
	return ws.FlushDetail{
		RolledBack: outcome.RolledBack,
		Bridge:     string(outcome.Bridge),
		Bytes:      outcome.Bytes,
	}, nil
}

func hasPendingChanges(cacheDir string) bool {
	return changes.HasPending(cacheDir)
}

func (s *Server) convertDocument(ctx context.Context, conv *convert.Converter, cacheDir, outPath, ext string) (convert.SaveResult, error) {
	if hasPendingChanges(cacheDir) {
		// A previous persist may have dropped Editor.bin (afterPersistCacheUpdate removes it
		// when no editor session is open) while new change blobs were still arriving. Without
		// a base, x2t cannot apply the delta and the save fails with "Editor.bin missing" —
		// which surfaces as a lost edit. Rebuild the base from the last persisted file first.
		if err := s.ensureEditorBinBase(ctx, conv, cacheDir, outPath); err != nil {
			return convert.SaveResult{}, err
		}
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("convert from changes", "cache", cacheDir, "ext", ext, "out", outPath)
		}
		res, err := conv.SaveChangesDetailed(ctx, cacheDir, outPath, ext)
		if err != nil {
			if s.opts.Logger != nil {
				s.opts.Logger.Warn("save changes conversion failed", "path", outPath, "err", err)
			}
			return convert.SaveResult{}, err
		}
		return res, nil
	}
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("convert from Editor.bin only", "cache", cacheDir, "ext", ext)
	}
	if err := s.ensureEditorBinBase(ctx, conv, cacheDir, outPath); err != nil {
		return convert.SaveResult{}, err
	}
	if err := conv.FromEditorBin(ctx, cacheDir, outPath, ext); err != nil {
		return convert.SaveResult{}, err
	}
	return convert.SaveResult{}, nil
}

// ensureEditorBinBase guarantees cacheDir/Editor.bin exists so a delta save has a base.
// When the base is missing but a previously persisted file is available, the base is
// rebuilt from it. If there is nothing to rebuild from, the missing base is left to the
// caller's normal error path.
func (s *Server) ensureEditorBinBase(ctx context.Context, conv *convert.Converter, cacheDir, outPath string) error {
	if convert.EditorBinCached(cacheDir) {
		return nil
	}
	if !fileExists(outPath) {
		return nil
	}
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("rebuilding missing Editor.bin base from persisted file",
			"cache", cacheDir, "from", outPath)
	}
	invalidateEditorBinCache(cacheDir)
	if err := conv.ToEditorBin(ctx, outPath, cacheDir); err != nil {
		return fmt.Errorf("office: rebuild Editor.bin base for %s: %w", filepath.Base(cacheDir), err)
	}
	return nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}
