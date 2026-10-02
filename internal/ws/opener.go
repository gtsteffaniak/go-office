package ws

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
)

// Opener downloads and converts documents for coauthoring open.
type Opener struct {
	Converter *convert.Converter
	CacheDir  string
	Logger    *slog.Logger
	Saver     DocumentSaver
}

func documentOpenPacket(cmdType, status string, data any) (string, error) {
	return socketMessage(map[string]any{
		"type": "documentOpen",
		"data": map[string]any{
			"type":     cmdType,
			"status":   status,
			"data":     data,
			"openedAt": time.Now().UnixMilli(),
		},
	})
}

func (o *Opener) Open(ctx context.Context, origin, basePath, docKey string, cmd openCmd) ([]string, error) {
	if o == nil {
		pkt, err := documentOpenPacket(cmd.Command, "error", "document opener not configured")
		if err != nil {
			return nil, err
		}
		return []string{pkt}, nil
	}
	if cmd.URL == "" {
		pkt, err := documentOpenPacket(cmd.Command, "error", "missing document url")
		if err != nil {
			return nil, err
		}
		return []string{pkt}, nil
	}

	ext := strings.TrimPrefix(strings.ToLower(cmd.Format), ".")
	if ext == "" {
		ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(cmd.URL)), ".")
	}
	if ext == "" {
		ext = "doc"
	}

	outDir := filepath.Join(o.CacheDir, docKey)
	pending := hasPendingChanges(outDir)

	// Orphaned change blobs from a prior session must be applied before serving
	// Editor.bin; never flush in the background while opening or the client can load
	// a stale bin and race the in-flight x2t conversion.
	if pending {
		if err := o.flushPending(ctx, docKey, origin); err != nil && o.Logger != nil {
			o.Logger.Error("flush pending changes before open", "key", docKey, "err", err)
		}
	}

	// Drop stale coauthoring blobs from a prior session unless edits are still pending.
	if !hasPendingChanges(outDir) {
		clearChanges(outDir)
	}

	if cmd.URL != "" && (convert.EditorBinCached(outDir) || convert.BrowserOriginCached(outDir, ext)) {
		match, err := o.cacheMatchesURL(ctx, outDir, cmd.URL)
		if err != nil {
			return o.errorPackets(cmd.Command, err)
		}
		if !match {
			invalidateOpenCache(outDir, ext)
		}
	}

	if packets, ok, err := o.openFromCache(cmd, origin, basePath, docKey, ext, outDir); ok || err != nil {
		if err != nil {
			return o.errorPackets(cmd.Command, err)
		}
		return packets, nil
	}

	tmp, err := os.CreateTemp("", "go-office-src-*."+ext)
	if err != nil {
		return o.errorPackets(cmd.Command, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err = downloadURL(ctx, cmd.URL, tmp); err != nil {
		return o.errorPackets(cmd.Command, err)
	}

	if convert.IsBrowserEditorFormat(ext) {
		return o.openBrowserDocument(cmd, origin, basePath, docKey, ext, tmpPath, outDir)
	}
	if o.Converter == nil {
		pkt, pktErr := documentOpenPacket(cmd.Command, "error", "document converter not configured")
		if pktErr != nil {
			return nil, pktErr
		}
		return []string{pkt}, nil
	}
	if err = o.Converter.ToEditorBin(ctx, tmpPath, outDir); err != nil {
		return o.errorPackets(cmd.Command, err)
	}
	return o.editorBinOpenPackets(cmd.Command, origin, basePath, docKey, outDir, "")
}

func (o *Opener) openFromCache(cmd openCmd, origin, basePath, docKey, ext, outDir string) ([]string, bool, error) {
	if convert.IsBrowserEditorFormat(ext) {
		if !convert.BrowserOriginCached(outDir, ext) {
			return nil, false, nil
		}
		cacheName := "origin." + ext
		files := map[string]string{
			cacheName: fileURL(origin, basePath, docKey, cacheName),
		}
		pkt, err := documentOpenPacket(cmd.Command, "ok", files)
		if err != nil {
			return nil, true, err
		}
		if o.Logger != nil {
			o.Logger.Info("document open ok (cached origin)", "key", docKey, "format", ext)
		}
		return []string{pkt}, true, nil
	}
	if !convert.EditorBinCached(outDir) {
		return nil, false, nil
	}
	packets, err := o.editorBinOpenPackets(cmd.Command, origin, basePath, docKey, outDir, "cached editor bin")
	if err != nil {
		return nil, true, err
	}
	return packets, true, nil
}

func (o *Opener) editorBinOpenPackets(cmdType, origin, basePath, docKey, outDir, logSuffix string) ([]string, error) {
	files := map[string]string{
		"Editor.bin": fileURL(origin, basePath, docKey, "Editor.bin"),
	}
	if entries, readErr := os.ReadDir(filepath.Join(outDir, "media")); readErr == nil {
		for _, ent := range entries {
			if ent.IsDir() {
				continue
			}
			name := "media/" + ent.Name()
			files[name] = fileURL(origin, basePath, docKey, name)
		}
	}
	pkt, err := documentOpenPacket(cmdType, "ok", files)
	if err != nil {
		return nil, err
	}
	if o.Logger != nil {
		msg := "document open ok"
		if logSuffix != "" {
			msg += " (" + logSuffix + ")"
		}
		if st, err := os.Stat(filepath.Join(outDir, "Editor.bin")); err == nil {
			o.Logger.Info(msg, "key", docKey, "editorBinBytes", st.Size())
		} else if logSuffix == "cached editor bin" {
			o.Logger.Info(msg, "key", docKey)
		}
	}
	return []string{pkt}, nil
}

func (o *Opener) openBrowserDocument(cmd openCmd, origin, basePath, docKey, ext, srcPath, outDir string) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return o.errorPackets(cmd.Command, err)
	}
	cacheName := "origin." + ext
	destPath := filepath.Join(outDir, cacheName)
	if err := copyFile(srcPath, destPath); err != nil {
		return o.errorPackets(cmd.Command, err)
	}
	files := map[string]string{
		cacheName: fileURL(origin, basePath, docKey, cacheName),
	}
	pkt, err := documentOpenPacket(cmd.Command, "ok", files)
	if err != nil {
		return nil, err
	}
	if o.Logger != nil {
		if st, err := os.Stat(destPath); err == nil {
			o.Logger.Info("document open ok", "key", docKey, "originBytes", st.Size(), "format", ext)
		}
	}
	return []string{pkt}, nil
}

func (o *Opener) flushPending(ctx context.Context, docKey, origin string) error {
	if o == nil || o.Saver == nil || o.CacheDir == "" || docKey == "" {
		return nil
	}
	if !hasPendingChanges(filepath.Join(o.CacheDir, docKey)) {
		return nil
	}
	flushCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	return o.Saver.FlushDocument(flushCtx, docKey, origin, false)
}

func invalidateOpenCache(outDir, ext string) {
	_ = os.Remove(filepath.Join(outDir, "Editor.bin"))
	_ = os.Remove(filepath.Join(outDir, "Editor.bin.part"))
	_ = os.Remove(filepath.Join(outDir, "source.sha256"))
	if convert.IsBrowserEditorFormat(ext) {
		_ = os.Remove(filepath.Join(outDir, "origin."+strings.TrimPrefix(strings.ToLower(ext), ".")))
	}
}

func (o *Opener) cacheMatchesURL(ctx context.Context, outDir, rawURL string) (bool, error) {
	tmp, err := os.CreateTemp("", "go-office-hash-*")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err = downloadURL(ctx, rawURL, tmp); err != nil {
		return false, err
	}
	hash, err := convert.FileSHA256(tmpPath)
	if err != nil {
		return false, err
	}
	return convert.EditorBinReusable(outDir, hash) || convert.SourceHashMatches(outDir, hash), nil
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

func (o *Opener) errorPackets(cmdType string, err error) ([]string, error) {
	if o.Logger != nil {
		o.Logger.Error("document open failed", "err", err)
	}
	pkt, perr := documentOpenPacket(cmdType, "error", err.Error())
	if perr != nil {
		return nil, err
	}
	return []string{pkt}, nil
}

func fileURL(origin, basePath, docKey, name string) string {
	origin = strings.TrimSuffix(origin, "/")
	basePath = strings.TrimSuffix(basePath, "/")
	// Normalise to either "" or "/prefix". Prepending "/" unconditionally turned an empty
	// base path into "/", which produced "//cache/files/..." (a doubled slash) and cost an
	// extra 307 round trip per cached resource in the editor.
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return origin + basePath + "/cache/files/" + url.PathEscape(docKey) + "/" + escapePathSegments(name)
}

func escapePathSegments(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func downloadURL(ctx context.Context, rawURL string, dest *os.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", rawURL, resp.Status)
	}
	_, err = io.Copy(dest, io.LimitReader(resp.Body, 128<<20))
	if err != nil {
		return err
	}
	return dest.Close()
}

func requestOrigin(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		host := r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
		return proto + "://" + host
	}
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

// CoauthoringOrigin returns the public document-server origin for cache URLs.
func CoauthoringOrigin(publicOrigin string, r *http.Request) string {
	if o := strings.TrimSuffix(strings.TrimSpace(publicOrigin), "/"); o != "" {
		return o
	}
	if r != nil {
		if o := strings.TrimSuffix(requestOrigin(r), "/"); o != "" {
			return o
		}
	}
	return ""
}
