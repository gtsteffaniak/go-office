package assetfetch

import (
	"os"
	"path/filepath"
	"strings"
)

// validAssetDirCore reports whether dir has a complete asset tree, excluding spellcheck languages.
func validAssetDirCore(dir string) bool {
	if dir == "" {
		return false
	}
	if needsConverterBin(dir) || !FontsReady(dir) || !SpellEngineReady(dir) {
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

// ValidAssetDir reports whether dir contains a complete Euro-Office asset tree (any version).
func ValidAssetDir(dir string) bool {
	return validAssetDirCore(dir) && DictionariesReady(dir)
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
