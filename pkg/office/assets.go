package office

import (
	"os"
	"path/filepath"
	"strings"
)

// AssetDirFromEnv returns OFFICE_ASSETS, or fallback when unset.
func AssetDirFromEnv(fallback string) string {
	if v := strings.TrimSpace(os.Getenv("OFFICE_ASSETS")); v != "" {
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
