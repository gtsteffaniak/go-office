package convert

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// prepareChangesForSave rewrites coauthoring blobs into the on-disk layout x2t expects
// before apply_changes (Document Server writes changes0.json via processChangesBase64).
func prepareChangesForSave(changesDir, _ string) error {
	jsonPath := filepath.Join(changesDir, "changes0.json")
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var blobs []string
	if err := json.Unmarshal(raw, &blobs); err != nil {
		return fmt.Errorf("convert: parse %s: %w", jsonPath, err)
	}
	if len(blobs) == 0 {
		return nil
	}

	// Match Document Server JSON concatenation (processChangesBase64).
	var b strings.Builder
	b.WriteByte('[')
	for i, blob := range blobs {
		if i > 0 {
			b.WriteByte(',')
		}
		enc, err := json.Marshal(blob)
		if err != nil {
			return err
		}
		b.Write(enc)
	}
	b.WriteByte(']')
	return os.WriteFile(jsonPath, []byte(b.String()), 0o644)
}
