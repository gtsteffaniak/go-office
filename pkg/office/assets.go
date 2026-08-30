package office

import (
	"os"
	"path/filepath"
	"strings"
)

// AssetDirFromEnv returns OFFICE_ASSETS, or the deprecated GO_OFFICE_ASSETS alias, then fallback.
func AssetDirFromEnv(fallback string) string {
	if v := strings.TrimSpace(os.Getenv("OFFICE_ASSETS")); v != "" {
		return v
	}
	// Deprecated: use OFFICE_ASSETS (see migration.md).
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
