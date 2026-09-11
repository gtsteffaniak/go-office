package convert

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const sourceHashFileName = "source.sha256"

// FileSHA256 returns the hex SHA-256 digest of path contents.
func FileSHA256(path string) (string, error) {
	return fileSHA256(path)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sourceHashPath(outDir string) string {
	return filepath.Join(outDir, sourceHashFileName)
}

// EditorBinReusable reports whether outDir has Editor.bin built from sourceHash.
func EditorBinReusable(outDir, sourceHash string) bool {
	return editorBinReusable(outDir, sourceHash)
}

func editorBinReusable(outDir, sourceHash string) bool {
	if sourceHash == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(outDir, "Editor.bin"))
	if err != nil || st.Size() == 0 {
		return false
	}
	got, err := os.ReadFile(sourceHashPath(outDir))
	if err != nil {
		return false
	}
	return string(got) == sourceHash
}

// EditorBinCached reports whether outDir already has a non-empty Editor.bin (document key binds content).
func EditorBinCached(outDir string) bool {
	st, err := os.Stat(filepath.Join(outDir, "Editor.bin"))
	return err == nil && st.Size() > 0
}

// BrowserOriginCached reports whether a browser-native document was copied to the cache.
func BrowserOriginCached(outDir, ext string) bool {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	if ext == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(outDir, "origin."+ext))
	return err == nil && st.Size() > 0
}

// SourceHashMatches reports whether outDir/source.sha256 equals sourceHash.
func SourceHashMatches(outDir, sourceHash string) bool {
	if sourceHash == "" {
		return false
	}
	got, err := os.ReadFile(sourceHashPath(outDir))
	if err != nil {
		return false
	}
	return string(got) == sourceHash
}

// WriteSourceHash records the source file digest beside a warmed cache entry.
func WriteSourceHash(outDir, sourceHash string) error {
	return writeSourceHash(outDir, sourceHash)
}

func writeSourceHash(outDir, sourceHash string) error {
	if sourceHash == "" {
		return nil
	}
	return os.WriteFile(sourceHashPath(outDir), []byte(sourceHash), 0o644)
}
