package convert

import "sync"

var (
	dirLocksMu sync.Mutex
	dirLocks   = map[string]*sync.Mutex{}
)

func mutexForCacheDir(dir string) *sync.Mutex {
	dirLocksMu.Lock()
	defer dirLocksMu.Unlock()
	if mu, ok := dirLocks[dir]; ok {
		return mu
	}
	mu := &sync.Mutex{}
	dirLocks[dir] = mu
	return mu
}

// withCacheDirLock serializes x2t work on a per-document cache directory.
// Parallel opens of the same sample (e.g. open-formats.spec.ts across workers)
// share one cache key; without this, concurrent ToEditorBin calls corrupt Editor.bin.
func withCacheDirLock(dir string, fn func() error) error {
	if dir == "" {
		return fn()
	}
	mu := mutexForCacheDir(dir)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}
