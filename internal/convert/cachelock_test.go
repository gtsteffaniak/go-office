package convert

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestWithCacheDirLockSerializesSameDir(t *testing.T) {
	dir := t.TempDir()
	var concurrent int32
	var peak int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = withCacheDirLock(dir, func() error {
				cur := atomic.AddInt32(&concurrent, 1)
				for {
					old := atomic.LoadInt32(&peak)
					if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
						break
					}
				}
				atomic.AddInt32(&concurrent, -1)
				return nil
			})
		}()
	}
	wg.Wait()
	if peak != 1 {
		t.Fatalf("peak concurrent holders = %d, want 1", peak)
	}
}
