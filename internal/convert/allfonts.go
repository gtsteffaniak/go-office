package convert

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var quotedAbsPathRe = regexp.MustCompile(`"(/[^"]+)"`)

// RewriteAllFontsPaths remaps quoted font paths in AllFonts.js so they resolve under
// assetDir. allfontsgen bakes in absolute paths from the host that ran fetch-assets.
func RewriteAllFontsPaths(src []byte, assetDir string) []byte {
	return rewriteAllFontsPaths(src, assetDir)
}

// rewriteAllFontsPaths remaps quoted font paths in converter AllFonts.js so they
// resolve under the current assetDir. font_selection.bin is left unchanged
// (length-changing rewrites corrupt it).
func rewriteAllFontsPaths(src []byte, assetDir string) []byte {
	if len(src) == 0 || assetDir == "" {
		return src
	}
	index := coreFontIndex(filepath.Join(assetDir, "core-fonts"))
	return quotedAbsPathRe.ReplaceAllFunc(src, func(m []byte) []byte {
		inner := string(m[1 : len(m)-1])
		if !looksLikeFontFilePath(inner) {
			return m
		}
		next := remapFontFilePath(inner, assetDir, index)
		if next == inner {
			return m
		}
		return []byte(`"` + next + `"`)
	})
}

func remapFontFilePath(p, assetDir string, index map[string]string) string {
	p = filepath.ToSlash(p)
	coreRoot := filepath.ToSlash(filepath.Join(assetDir, "core-fonts"))
	const marker = "/core-fonts/"
	if i := strings.Index(p, marker); i >= 0 {
		return coreRoot + "/" + p[i+len(marker):]
	}
	if strings.Contains(p, "/share/fonts/") {
		base := strings.ToLower(filepath.Base(p))
		if mapped, ok := index[base]; ok {
			return filepath.ToSlash(mapped)
		}
	}
	return p
}

func looksLikeFontFilePath(p string) bool {
	if !strings.HasPrefix(p, "/") {
		return false
	}
	lower := strings.ToLower(p)
	switch {
	case strings.Contains(p, "/core-fonts/"):
		return true
	case strings.Contains(p, "/share/fonts/"):
		return true
	case strings.HasSuffix(lower, ".ttf"), strings.HasSuffix(lower, ".ttc"), strings.HasSuffix(lower, ".otf"):
		return true
	default:
		return false
	}
}

func coreFontIndex(coreDir string) map[string]string {
	index := make(map[string]string)
	_ = filepath.Walk(coreDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		lower := strings.ToLower(info.Name())
		if !strings.HasSuffix(lower, ".ttf") && !strings.HasSuffix(lower, ".ttc") && !strings.HasSuffix(lower, ".otf") {
			return nil
		}
		if _, exists := index[lower]; !exists {
			index[lower] = path
		}
		return nil
	})
	return index
}

func allFontFilePaths(src []byte) []string {
	matches := quotedAbsPathRe.FindAllSubmatch(src, -1)
	seen := make(map[string]struct{})
	var out []string
	for _, m := range matches {
		p := string(m[1])
		if !looksLikeFontFilePath(p) {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

