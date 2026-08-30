package ws

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// appendChanges writes editor change blobs under cacheDir/changes/{time}.json.
func appendChanges(cacheDir string, changes []string) (int, error) {
	if len(changes) == 0 {
		return maxChangeIndex(cacheDir)
	}
	dir := filepath.Join(cacheDir, "changes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	name := fmt.Sprintf("%d.json", time.Now().UnixMilli())
	raw, err := json.Marshal(changes)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
		return 0, err
	}
	return maxChangeIndex(cacheDir)
}

func maxChangeIndex(cacheDir string) (int, error) {
	entries, err := os.ReadDir(filepath.Join(cacheDir, "changes"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return len(entries), nil
}

func clearChanges(cacheDir string) {
	_ = os.RemoveAll(filepath.Join(cacheDir, "changes"))
}

func parseChanges(msg map[string]any) []string {
	raw, ok := msg["changes"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		var out []string
		if json.Unmarshal([]byte(v), &out) == nil {
			return out
		}
		return []string{v}
	default:
		return nil
	}
}
