package office

import "sync"

var (
	converterLocksMu sync.Mutex
	converterLocks   = make(map[string]*sync.Mutex)
)

func lockConverter(cacheName string) func() {
	converterLocksMu.Lock()
	mu, ok := converterLocks[cacheName]
	if !ok {
		mu = &sync.Mutex{}
		converterLocks[cacheName] = mu
	}
	converterLocksMu.Unlock()

	mu.Lock()
	return mu.Unlock
}
