package ws

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const changesFileName = "changes0.json"

// appendChanges appends editor change blobs to cacheDir/changes/changes0.json.
// x2t expects this exact file when m_bFromChanges is true.
func appendChanges(cacheDir string, changes []string) (int, error) {
	if len(changes) == 0 {
		return maxChangeIndex(cacheDir)
	}
	dir := filepath.Join(cacheDir, "changes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}

	path := filepath.Join(dir, changesFileName)
	existing, err := readChangesFile(path)
	if err != nil {
		return 0, err
	}
	existing = append(existing, changes...)
	raw, err := json.Marshal(existing)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return 0, err
	}
	return len(existing), nil
}

func readChangesFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var existing []string
	if err := json.Unmarshal(data, &existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func maxChangeIndex(cacheDir string) (int, error) {
	existing, err := readChangesFile(filepath.Join(cacheDir, "changes", changesFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return len(existing), nil
}

func clearChanges(cacheDir string) {
	_ = os.RemoveAll(filepath.Join(cacheDir, "changes"))
}

func hasPendingChanges(cacheDir string) bool {
	n, err := maxChangeIndex(cacheDir)
	return err == nil && n > 0
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

func changeBlobPreview(blobs []string) []string {
	out := make([]string, 0, len(blobs))
	for _, blob := range blobs {
		out = append(out, truncateRunes(decodeChangeBlob(blob), 80))
	}
	return out
}

func decodeChangeBlob(blob string) string {
	payload := blob
	if i := strings.IndexByte(blob, ';'); i >= 0 {
		payload = blob[i+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(raw) < 2 {
		return blob
	}
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1]
	}
	u := make([]uint16, len(raw)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	s := string(utf16.Decode(u))
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r != 0xFFFD {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return blob
	}
	return b.String()
}

func truncateRunes(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	i := 0
	for idx := range s {
		if i == n {
			return s[:idx]
		}
		i++
	}
	return s
}
