package convert

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Options configures the x2t subprocess converter.
type Options struct {
	AssetDir string
	Limit    int
}

// Converter runs Euro-Office x2t to produce Editor.bin from office files.
type Converter struct {
	binDir   string
	fontDir  string
	themeDir string
	limit    chan struct{}
}

// New creates a converter. Returns an error when x2t is missing.
func New(opts Options) (*Converter, error) {
	if opts.AssetDir == "" {
		return nil, fmt.Errorf("convert: asset dir is required")
	}
	binDir := filepath.Join(opts.AssetDir, "converter", "bin")
	x2t := filepath.Join(binDir, "x2t")
	if st, err := os.Stat(x2t); err != nil || st.IsDir() {
		return nil, fmt.Errorf("convert: x2t not found at %s (run make build)", x2t)
	}
	if err := ensureExecutable(x2t); err != nil {
		slog.Warn("x2t ensure executable", "err", err)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 1
	}
	fontDir := filepath.Join(opts.AssetDir, "core-fonts")
	if st, err := os.Stat(fontDir); err != nil || !st.IsDir() {
		fontDir = filepath.Join(opts.AssetDir, "fonts")
	}
	return &Converter{
		binDir:   binDir,
		fontDir:  fontDir,
		themeDir: filepath.Join(opts.AssetDir, "sdkjs", "slide", "themes"),
		limit:    make(chan struct{}, limit),
	}, nil
}

// ToEditorBin converts sourcePath into outDir/Editor.bin.
func (c *Converter) ToEditorBin(ctx context.Context, sourcePath, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	outFile := filepath.Join(outDir, "Editor.bin")
	if st, err := os.Stat(outFile); err == nil && st.Size() > 0 {
		return nil
	}

	select {
	case c.limit <- struct{}{}:
		defer func() { <-c.limit }()
	case <-ctx.Done():
		return ctx.Err()
	}

	taskFile, err := os.CreateTemp("", "go-office-x2t-*.xml")
	if err != nil {
		return err
	}
	taskPath := taskFile.Name()
	defer os.Remove(taskPath)

	xml := buildTaskXML(sourcePath, outFile, c.fontDir, c.themeDir)
	if _, err := taskFile.WriteString(xml); err != nil {
		taskFile.Close()
		return err
	}
	if err := taskFile.Close(); err != nil {
		return err
	}

	out, err := c.runX2t(ctx, taskPath)
	if err != nil {
		return fmt.Errorf("convert: x2t: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if st, err := os.Stat(outFile); err != nil || st.Size() == 0 {
		return fmt.Errorf("convert: x2t produced no output")
	}
	return nil
}

// runX2t executes x2t via /bin/sh so chmod and LD_LIBRARY_PATH match Nextcloud's approach.
func (c *Converter) runX2t(ctx context.Context, taskPath string) ([]byte, error) {
	script := fmt.Sprintf("chmod +x ./x2t 2>/dev/null; LD_LIBRARY_PATH=. exec ./x2t %s", shellQuote(taskPath))
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
	cmd.Dir = c.binDir
	return cmd.CombinedOutput()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode()
	if mode&0o111 != 0 {
		return nil
	}
	if err := os.Chmod(path, mode.Perm()|0o755); err != nil {
		slog.Error("x2t chmod failed", "path", path, "err", err)
		return err
	}
	after, err := os.Stat(path)
	if err != nil {
		return err
	}
	if after.Mode()&0o111 == 0 {
		return fmt.Errorf("chmod had no effect on %s (perm=%s)", path, after.Mode().Perm())
	}
	slog.Info("x2t chmod applied", "path", path, "perm", after.Mode().Perm())
	return nil
}

func buildTaskXML(from, to, fontDir, themeDir string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<TaskQueueDataConvert xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
<m_sFileFrom>%s</m_sFileFrom>
<m_sFileTo>%s</m_sFileTo>
<m_nFormatTo>%d</m_nFormatTo>
<m_sFontDir>%s</m_sFontDir>
<m_sThemeDir>%s</m_sThemeDir>
<m_oTimestamp>%s</m_oTimestamp>
</TaskQueueDataConvert>
`, escapeXML(from), escapeXML(to), FormatCanvas, escapeXML(fontDir), escapeXML(themeDir), now)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
