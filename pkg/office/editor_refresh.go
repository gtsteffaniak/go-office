package office

import (
	"os"
	"path/filepath"

	"github.com/quantumx-apps/go-office/internal/ws"
)

// afterPersistCacheUpdate drops stale Editor.bin after a successful persist when
// no editor session is open. Active sessions need Editor.bin for the next
// coauthoring saveChanges round; invalidating under load caused spurious flush
// failures and editor save errors.
func (s *Server) afterPersistCacheUpdate(cacheDir string) {
	docKey := filepath.Base(cacheDir)
	if ws.HasActiveDocumentSession(docKey) {
		if s.opts.Logger != nil {
			s.opts.Logger.Debug("keeping Editor.bin while session active", "key", docKey)
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
