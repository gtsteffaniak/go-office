package convert

import "sync"

var cacheDirLocks sync.Map // cache dir path -> *sync.Mutex

// withCacheDirLock serializes x2t work on a per-document cache directory.
// Parallel opens of the same sample (e.g. open-formats.spec.ts across workers)
// share one cache key; without this, concurrent ToEditorBin calls corrupt Editor.bin.
func withCacheDirLock(dir string, fn func() error) error {
	if dir == "" {
		return fn()
	}
	v, _ := cacheDirLocks.LoadOrStore(dir, &sync.Mutex{})
	mu, ok := v.(*sync.Mutex)
	if !ok {
		return fn()
	}
	mu.Lock()
	defer func() {
		mu.Unlock()
		cacheDirLocks.Delete(dir)
	}()
	return fn()
}
