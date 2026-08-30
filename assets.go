package office

import (
	"os"
	"path/filepath"
	"strings"
)

// AssetDirFromEnv returns the asset directory from GO_OFFICE_ASSETS, or fallback when set.
func AssetDirFromEnv(fallback string) string {
	if v := strings.TrimSpace(os.Getenv("GO_OFFICE_ASSETS")); v != "" {
		return v
	}
	return fallback
}

// ReadAssetVersion reads assets/VERSION written by scripts/fetch-assets.*.
func ReadAssetVersion(assetDir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(assetDir, "VERSION"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
