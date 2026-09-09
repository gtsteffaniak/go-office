package office

import "sync"

var converterLocks sync.Map // map[string]*sync.Mutex keyed by cache dir name

func lockConverter(cacheName string) func() {
	mu, _ := converterLocks.LoadOrStore(cacheName, &sync.Mutex{})
	m := mu.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}
