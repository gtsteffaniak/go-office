package office

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/quantumx-apps/go-office/internal/convert"
)

// EnsureEditorBin prepares cache/{docKey}/Editor.bin from a local source file.
// Cache hits return immediately; concurrent callers serialize via the convert cache lock.
func (s *Server) EnsureEditorBin(ctx context.Context, docKey, sourcePath, ext string) error {
	docKey = strings.TrimSpace(docKey)
	sourcePath = strings.TrimSpace(sourcePath)
	if docKey == "" || sourcePath == "" {
		return fmt.Errorf("office: warm: missing document key or source path")
	}
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	outDir := filepath.Join(s.cacheDir(), docKey)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	srcHash, err := convert.FileSHA256(sourcePath)
	if err != nil {
		return err
	}
	if convert.IsBrowserEditorFormat(ext) {
		dest := filepath.Join(outDir, "origin."+ext)
		if convert.BrowserOriginCached(outDir, ext) && convert.SourceHashMatches(outDir, srcHash) {
			return nil
		}
		if err = copyFile(sourcePath, dest); err != nil {
			return err
		}
		return convert.WriteSourceHash(outDir, srcHash)
	}
	if convert.EditorBinReusable(outDir, srcHash) {
		return nil
	}
	if convert.EditorBinCached(outDir) {
		invalidateEditorBinCache(outDir)
	}
	conv, err := s.converter()
	if err != nil {
		return err
	}
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("warm convert start", "key", docKey, "ext", ext)
	}
	err = conv.ToEditorBinLow(ctx, sourcePath, outDir)
	if err != nil {
		return err
	}
	if s.opts.Logger != nil {
		s.opts.Logger.Debug("warm convert ok", "key", docKey, "ext", ext)
	}
	return nil
}

// EditorBinFresh reports whether cache/{docKey} matches the current source file bytes.
func (s *Server) EditorBinFresh(docKey, sourcePath, ext string) bool {
	docKey = strings.TrimSpace(docKey)
	sourcePath = strings.TrimSpace(sourcePath)
	if docKey == "" || sourcePath == "" {
		return false
	}
	srcHash, err := convert.FileSHA256(sourcePath)
	if err != nil {
		return false
	}
	outDir := filepath.Join(s.cacheDir(), docKey)
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	if convert.IsBrowserEditorFormat(ext) {
		return convert.BrowserOriginCached(outDir, ext) && convert.SourceHashMatches(outDir, srcHash)
	}
	return convert.EditorBinReusable(outDir, srcHash)
}

// EditorBinCached reports whether cache/{docKey} already has a usable Editor.bin.
func (s *Server) EditorBinCached(docKey string) bool {
	docKey = strings.TrimSpace(docKey)
	if docKey == "" {
		return false
	}
	return convert.EditorBinCached(filepath.Join(s.cacheDir(), docKey))
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
