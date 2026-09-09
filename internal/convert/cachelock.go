package convert

import (
	"github.com/quantumx-apps/go-office/internal/changes"
)

// withCacheDirLock serializes x2t work on a per-document cache directory.
// Parallel opens of the same sample (e.g. open-formats.spec.ts across workers)
// share one cache key; without this, concurrent ToEditorBin calls corrupt Editor.bin.
func withCacheDirLock(dir string, fn func() error) error {
	return changes.WithDirLock(dir, fn)
}
