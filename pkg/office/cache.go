package office

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultCacheTTL       = 24 * time.Hour
	defaultCacheJanitor   = 15 * time.Minute
	defaultCacheMaxEntries = 256
)

type cacheJanitor struct {
	mu       sync.Mutex
	dir      string
	ttl      time.Duration
	maxDirs  int
	ticker   *time.Ticker
	stopCh   chan struct{}
	stopped  bool
	onRemove func(docKey string)
}

func newCacheJanitor(dir string, ttl time.Duration, maxDirs int, onRemove func(string)) *cacheJanitor {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	if maxDirs <= 0 {
		maxDirs = defaultCacheMaxEntries
	}
	return &cacheJanitor{
		dir:      dir,
		ttl:      ttl,
		maxDirs:  maxDirs,
		onRemove: onRemove,
	}
}

func (j *cacheJanitor) start() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.ticker != nil || j.stopped {
		return
	}
	j.ticker = time.NewTicker(defaultCacheJanitor)
	j.stopCh = make(chan struct{})
	go j.loop()
}

func (j *cacheJanitor) loop() {
	for {
		select {
		case <-j.ticker.C:
			j.sweep()
		case <-j.stopCh:
			return
		}
	}
}

func (j *cacheJanitor) stop() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.stopped {
		return
	}
	j.stopped = true
	if j.ticker != nil {
		j.ticker.Stop()
	}
	if j.stopCh != nil {
		close(j.stopCh)
	}
}

func (j *cacheJanitor) sweep() {
	entries, err := os.ReadDir(j.dir)
	if err != nil {
		return
	}
	now := time.Now()
	type item struct {
		name    string
		modTime time.Time
	}
	var dirs []item
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		dirs = append(dirs, item{name: ent.Name(), modTime: info.ModTime()})
	}
	for _, d := range dirs {
		if now.Sub(d.modTime) > j.ttl {
			j.remove(d.name)
		}
	}
	if len(dirs) <= j.maxDirs {
		return
	}
	// Evict oldest when over limit.
	for len(dirs) > j.maxDirs {
		oldest := 0
		for i := 1; i < len(dirs); i++ {
			if dirs[i].modTime.Before(dirs[oldest].modTime) {
				oldest = i
			}
		}
		j.remove(dirs[oldest].name)
		dirs = append(dirs[:oldest], dirs[oldest+1:]...)
	}
}

func (j *cacheJanitor) remove(docKey string) {
	if docKey == "" {
		return
	}
	_ = os.RemoveAll(filepath.Join(j.dir, docKey))
	if j.onRemove != nil {
		j.onRemove(docKey)
	}
}

func cacheDirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func (s *Server) removeCacheDir(docKey string) {
	if s.cacheJanitor != nil {
		s.cacheJanitor.remove(docKey)
		return
	}
	_ = os.RemoveAll(filepath.Join(s.cacheDir(), docKey))
}

// NewCacheJanitorForTest creates a janitor for unit tests.
func NewCacheJanitorForTest(dir string, ttl time.Duration, maxDirs int, onRemove func(string)) *cacheJanitor {
	return newCacheJanitor(dir, ttl, maxDirs, onRemove)
}

// SweepOnce runs a single cache cleanup pass.
func (j *cacheJanitor) SweepOnce() {
	j.sweep()
}

func cacheDirCount(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n++
		}
	}
	return n
}
