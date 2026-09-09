package changes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	dirLocksMu sync.Mutex
	dirLocks   = map[string]*sync.Mutex{}
)

func mutexForCacheDir(cacheDir string) *sync.Mutex {
	dirLocksMu.Lock()
	defer dirLocksMu.Unlock()
	if mu, ok := dirLocks[cacheDir]; ok {
		return mu
	}
	mu := &sync.Mutex{}
	dirLocks[cacheDir] = mu
	return mu
}

// WithDirLock serializes all change-journal and per-document x2t work on cacheDir.
func WithDirLock(cacheDir string, fn func() error) error {
	if cacheDir == "" {
		return fn()
	}
	mu := mutexForCacheDir(cacheDir)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

func changesPath(cacheDir string) string {
	return filepath.Join(cacheDir, "changes", FileName)
}

func readBlobs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var blobs []string
	if err := json.Unmarshal(data, &blobs); err != nil {
		return nil, err
	}
	return blobs, nil
}

func writeBlobsAtomic(path string, blobs []string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(blobs)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".changes-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err = tmp.Write(raw); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err = os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// Append adds coauthoring change blobs to cacheDir/changes/changes0.json.
func Append(cacheDir string, blobs []string) (int, error) {
	if len(blobs) == 0 {
		return Count(cacheDir)
	}
	var count int
	err := WithDirLock(cacheDir, func() error {
		path := changesPath(cacheDir)
		existing, err := readBlobs(path)
		if err != nil {
			return err
		}
		existing = append(existing, blobs...)
		if err := writeBlobsAtomic(path, existing); err != nil {
			return err
		}
		count = len(existing)
		return nil
	})
	return count, err
}

// Count returns the number of change blobs in the journal.
func Count(cacheDir string) (int, error) {
	var count int
	err := WithDirLock(cacheDir, func() error {
		blobs, err := readBlobs(changesPath(cacheDir))
		if err != nil {
			return err
		}
		count = len(blobs)
		return nil
	})
	return count, err
}

// Clear removes all pending change blobs.
func Clear(cacheDir string) error {
	return WithDirLock(cacheDir, func() error {
		return os.RemoveAll(filepath.Join(cacheDir, "changes"))
	})
}

// Acknowledge removes the first throughCount blobs converted in the last flush.
// Blobs appended during conversion are preserved.
func Acknowledge(cacheDir string, throughCount int) error {
	if throughCount <= 0 {
		return nil
	}
	return WithDirLock(cacheDir, func() error {
		path := changesPath(cacheDir)
		blobs, err := readBlobs(path)
		if err != nil {
			return err
		}
		if len(blobs) == 0 {
			return nil
		}
		if throughCount >= len(blobs) {
			return os.RemoveAll(filepath.Join(cacheDir, "changes"))
		}
		return writeBlobsAtomic(path, blobs[throughCount:])
	})
}

// Snapshot captures an immutable view of pending blobs for conversion.
type Snapshot struct {
	ChangesDir string
	BlobCount  int
}

// BeginSnapshot copies the current journal for x2t without blocking new appends
// after the copy completes.
func BeginSnapshot(cacheDir string) (Snapshot, error) {
	var snap Snapshot
	err := WithDirLock(cacheDir, func() error {
		src := changesPath(cacheDir)
		blobs, err := readBlobs(src)
		if err != nil {
			return err
		}
		if len(blobs) == 0 {
			return fmt.Errorf("changes: no blobs to snapshot in %s", cacheDir)
		}
		snapDir := filepath.Join(cacheDir, ".convert-snapshot")
		if err := os.RemoveAll(snapDir); err != nil {
			return err
		}
		changesDir := filepath.Join(snapDir, "changes")
		if err := os.MkdirAll(changesDir, 0o755); err != nil {
			return err
		}
		if err := writeBlobsAtomic(filepath.Join(changesDir, FileName), blobs); err != nil {
			return err
		}
		snap.ChangesDir = changesDir
		snap.BlobCount = len(blobs)
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// RemoveSnapshot drops the on-disk snapshot directory after conversion.
func RemoveSnapshot(cacheDir string) {
	_ = os.RemoveAll(filepath.Join(cacheDir, ".convert-snapshot"))
}
