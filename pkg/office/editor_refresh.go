package office

import (
	"context"
	"os"
	"path/filepath"

	"github.com/quantumx-apps/go-office/internal/ws"
)

// refreshEditorBinFromSaved rebuilds cacheDir/Editor.bin from a just-persisted file
// so the next saveChanges batch applies on the correct server-side base.
func (s *Server) refreshEditorBinFromSaved(ctx context.Context, cacheDir, savedPath string) error {
	conv, err := s.converter()
	if err != nil {
		return err
	}
	docKey := filepath.Base(cacheDir)
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("refresh Editor.bin after changes persist", "key", docKey, "from", savedPath)
	}
	invalidateEditorBinCache(cacheDir)
	return conv.ToEditorBin(ctx, savedPath, cacheDir)
}

// afterPersistCacheUpdate drops Editor.bin after persist when no editor session is
// open so reopen reconverts from storage. Active sessions keep the refreshed
// Editor.bin produced by refreshEditorBinFromSaved for the next delta round.
func (s *Server) afterPersistCacheUpdate(cacheDir string) {
	docKey := filepath.Base(cacheDir)
	if ws.HasActiveDocumentSession(docKey) {
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("keeping refreshed Editor.bin while session active", "key", docKey)
		}
		return
	}
	invalidateEditorBinCache(cacheDir)
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("invalidated Editor.bin after persist", "cache", cacheDir)
	}
}

func invalidateEditorBinCache(cacheDir string) {
	_ = os.Remove(filepath.Join(cacheDir, "Editor.bin"))
	_ = os.Remove(filepath.Join(cacheDir, "Editor.bin.part"))
	_ = os.Remove(filepath.Join(cacheDir, "source.sha256"))
}
