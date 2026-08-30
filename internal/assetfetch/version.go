package assetfetch

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Version pins a Euro-Office Document Server release.
type Version struct {
	Release  string // e.g. 9.3.4-hotfix.1 (no leading v)
	Protocol string // reported to sdkjs; defaults to Release
}

// LoadVersion reads scripts/euro-office.version.
func LoadVersion(path string) (Version, error) {
	f, err := os.Open(path)
	if err != nil {
		return Version{}, err
	}
	defer f.Close()

	var v Version
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
		case "EURO_OFFICE_RELEASE":
			v.Release = strings.TrimPrefix(val, "v")
		case "EURO_OFFICE_PROTOCOL":
			v.Protocol = val
		}
	}
	if err := sc.Err(); err != nil {
		return Version{}, err
	}
	if v.Release == "" {
		return Version{}, fmt.Errorf("EURO_OFFICE_RELEASE missing in %s", path)
	}
	if v.Protocol == "" {
		v.Protocol = v.Release
	}
	return v, nil
}

// DebURL returns the GitHub release .deb download URL for this Linux host arch.
func DebURL(v Version) string {
	arch := DebArch()
	return fmt.Sprintf(
		"https://github.com/Euro-Office/DocumentServer/releases/download/v%s/euro-office-documentserver_%s_%s.deb",
		v.Release, v.Release, arch,
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
