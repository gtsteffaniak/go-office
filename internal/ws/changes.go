package ws

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"unicode/utf16"

	docchanges "github.com/quantumx-apps/go-office/internal/changes"
)

const changesFileName = docchanges.FileName

func appendChanges(cacheDir string, blobs []string) (int, error) {
	return docchanges.Append(cacheDir, blobs)
}

func maxChangeIndex(cacheDir string) (int, error) {
	return docchanges.Count(cacheDir)
}

func clearChanges(cacheDir string) {
	_ = docchanges.Clear(cacheDir)
}

func hasPendingChanges(cacheDir string) bool {
	return docchanges.HasPending(cacheDir)
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
