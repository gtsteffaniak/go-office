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

	tmp, err := os.CreateTemp("", "go-office-src-*."+ext)
	if err != nil {
		return o.errorPackets(cmd.Command, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := downloadURL(ctx, cmd.URL, tmp); err != nil {
		return o.errorPackets(cmd.Command, err)
	}

	outDir := filepath.Join(o.CacheDir, docKey)
	if convert.IsBrowserEditorFormat(ext) {
		return o.openBrowserDocument(cmd, origin, basePath, docKey, ext, tmpPath, outDir)
	}
	if o.Converter == nil {
		pkt, err := documentOpenPacket(cmd.Command, "error", "document converter not configured")
		if err != nil {
			return nil, err
		}
		return []string{pkt}, nil
	}
	if err := o.Converter.ToEditorBin(ctx, tmpPath, outDir); err != nil {
		if o.Logger != nil {
			o.Logger.Error("document open failed", "key", docKey, "url", cmd.URL, "err", err)
		}
		return o.errorPackets(cmd.Command, err)
	}

	files := map[string]string{
		"Editor.bin": fileURL(origin, basePath, docKey, "Editor.bin"),
	}
	if entries, err := os.ReadDir(filepath.Join(outDir, "media")); err == nil {
		for _, ent := range entries {
			if ent.IsDir() {
				continue
			}
			name := "media/" + ent.Name()
			files[name] = fileURL(origin, basePath, docKey, name)
		}
	}

	pkt, err := documentOpenPacket(cmd.Command, "ok", files)
	if err != nil {
		return nil, err
	}
	if o.Logger != nil {
		if st, err := os.Stat(filepath.Join(outDir, "Editor.bin")); err == nil {
			o.Logger.Info("document open ok", "key", docKey, "editorBinBytes", st.Size())
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
		o.Logger.Warn("document open failed", "err", err)
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
	if !strings.HasPrefix(basePath, "/") {
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
	if o := strings.TrimSpace(publicOrigin); o != "" {
		return strings.TrimSuffix(o, "/")
	}
	return requestOrigin(r)
}
