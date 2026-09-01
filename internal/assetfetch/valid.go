package assetfetch

import (
	"os"
	"path/filepath"
	"strings"
)

// ValidAssetDir reports whether dir contains a complete Euro-Office asset tree (any version).
func ValidAssetDir(dir string) bool {
	if dir == "" {
		return false
	}
	if needsConverterBin(dir) || !FontsReady(dir) {
		return false
	}
	apiJs := filepath.Join(dir, "web-apps", "apps", "api", "documents", "api.js")
	apiTpl := filepath.Join(dir, "web-apps", "apps", "api", "documents", "api.js.tpl")
	if _, err := os.Stat(apiJs); err == nil {
		return true
	}
	if _, err := os.Stat(apiTpl); err == nil {
		return true
	}
	return false
}

// ReadInstalledVersion returns assets/VERSION or the .extracted marker when VERSION is absent.
func ReadInstalledVersion(dir string) string {
	if b, err := os.ReadFile(filepath.Join(dir, "VERSION")); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" {
			return v
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, ".extracted")); err == nil {
		return strings.TrimSpace(string(b))
	}
	return ""
}
