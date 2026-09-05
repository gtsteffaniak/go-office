package office

import (
	"os"
	"path/filepath"
)

// afterPersistCacheUpdate drops stale Editor.bin after a successful persist.
// Active sessions keep document state in memory; the next open reconverts from
// storage. Skipping a post-persist x2t refresh avoids competing with queued saves
// when OFFICE_CONVERT_LIMIT is saturated.
func (s *Server) afterPersistCacheUpdate(cacheDir string) {
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
