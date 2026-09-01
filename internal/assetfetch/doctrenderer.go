package assetfetch

import (
	"os"
	"path/filepath"
	"strings"
)

func fixDoctRendererConfig(binDir string) error {
	path := filepath.Join(binDir, "DoctRenderer.config")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	updated := strings.ReplaceAll(string(data), "../../../", "../../")
	if updated == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}
