package assetfetch

import (
	"os"
	"path/filepath"
)

// clearDirContents removes every entry inside dir but leaves dir itself.
// Unlike os.RemoveAll(dir), this works when dir is a mount point (e.g. Docker volume at /src/assets).
func clearDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var firstErr error
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
