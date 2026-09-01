package assetfetch

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LoadVersion reads EURO_OFFICE_VERSION from a euro-office.version pin file.
// Legacy EURO_OFFICE_RELEASE is accepted as an alias.
func LoadVersion(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var version string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "EURO_OFFICE_VERSION":
			version = strings.TrimPrefix(val, "v")
		case "EURO_OFFICE_RELEASE":
			if version == "" {
				version = strings.TrimPrefix(val, "v")
			}
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	if version == "" {
		return "", fmt.Errorf("EURO_OFFICE_VERSION missing in %s", path)
	}
	return version, nil
}

// DebURL returns the GitHub release .deb download URL for this Linux host arch.
func DebURL(version string) string {
	arch := DebArch()
	return fmt.Sprintf(
		"https://github.com/Euro-Office/DocumentServer/releases/download/v%s/euro-office-documentserver_%s_%s.deb",
		version, version, arch,
	)
}

// FindRepoRoot walks up from cwd until go.mod is found.
func FindRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

// FindDocumentServerRoot locates web-apps + sdkjs inside an extracted package tree.
func FindDocumentServerRoot(extractRoot string) (string, error) {
	candidates := []string{
		filepath.Join(extractRoot, "var", "www", "euro-office", "documentserver"),
		filepath.Join(extractRoot, "var", "www", "onlyoffice", "documentserver"),
	}
	for _, c := range candidates {
		if isDocumentServerRoot(c) {
			return c, nil
		}
	}

	var found string
	err := filepath.WalkDir(extractRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || d.Name() != "web-apps" {
			return nil
		}
		parent := filepath.Dir(path)
		if isDocumentServerRoot(parent) {
			found = parent
			return fs.SkipAll
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return "", err
	}
	if found != "" {
		return found, nil
	}
	return "", fmt.Errorf("documentserver root not found under %s", extractRoot)
}

func isDocumentServerRoot(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	for _, name := range []string{"web-apps", "sdkjs"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !st.IsDir() {
			return false
		}
	}
	return true
}
