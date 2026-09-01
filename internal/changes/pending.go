package changes

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileName is the coauthoring changes blob file x2t reads when m_bFromChanges is true.
const FileName = "changes0.json"

// HasPending reports whether cacheDir has non-empty coauthoring change blobs.
func HasPending(cacheDir string) bool {
	data, err := os.ReadFile(filepath.Join(cacheDir, "changes", FileName))
	if err != nil || len(data) == 0 {
		return false
	}
	var blobs []string
	if err := json.Unmarshal(data, &blobs); err != nil {
		return false
	}
	return len(blobs) > 0
}

// Count returns the number of change blobs in changes0.json.
func Count(cacheDir string) (int, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, "changes", FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	var blobs []string
	if err := json.Unmarshal(data, &blobs); err != nil {
		return 0, err
	}
	return len(blobs), nil
}
