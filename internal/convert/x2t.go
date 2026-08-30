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

	xml := buildTaskXML(sourcePath, outFile, c.fontDir, c.themeDir, filepath.Ext(sourcePath))
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

// FromEditorBin converts cacheDir/Editor.bin to destPath (e.g. saved.docx).
func (c *Converter) FromEditorBin(ctx context.Context, cacheDir, destPath, targetExt string) error {
	editorBin := filepath.Join(cacheDir, "Editor.bin")
	if st, err := os.Stat(editorBin); err != nil || st.Size() == 0 {
		return fmt.Errorf("convert: Editor.bin missing in %s", cacheDir)
	}
	return c.fromEditor(ctx, editorBin, destPath, targetExt, false)
}

// SaveChanges applies cacheDir/changes/*.json on top of Editor.bin and writes destPath.
func (c *Converter) SaveChanges(ctx context.Context, cacheDir, destPath, targetExt string) error {
	editorBin := filepath.Join(cacheDir, "Editor.bin")
	if st, err := os.Stat(editorBin); err != nil || st.Size() == 0 {
		return fmt.Errorf("convert: Editor.bin missing in %s", cacheDir)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	entries, err := os.ReadDir(changesDir)
	if err != nil || len(entries) == 0 {
		return c.FromEditorBin(ctx, cacheDir, destPath, targetExt)
	}
	return c.fromEditor(ctx, editorBin, destPath, targetExt, true)
}

func (c *Converter) fromEditor(ctx context.Context, editorBin, destPath, targetExt string, fromChanges bool) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	select {
	case c.limit <- struct{}{}:
		defer func() { <-c.limit }()
	case <-ctx.Done():
		return ctx.Err()
	}

	taskFile, err := os.CreateTemp("", "go-office-x2t-save-*.xml")
	if err != nil {
		return err
	}
	taskPath := taskFile.Name()
	defer os.Remove(taskPath)

	ext := strings.TrimPrefix(strings.ToLower(targetExt), ".")
	xml := buildReverseTaskXML(editorBin, destPath, c.fontDir, c.themeDir, ext, fromChanges)
	if _, err := taskFile.WriteString(xml); err != nil {
		taskFile.Close()
		return err
	}
	if err := taskFile.Close(); err != nil {
		return err
	}

	out, err := c.runX2t(ctx, taskPath)
	if err != nil {
		return fmt.Errorf("convert: reverse x2t: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if st, err := os.Stat(destPath); err != nil || st.Size() == 0 {
		return fmt.Errorf("convert: reverse x2t produced no output")
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

func buildTaskXML(from, to, fontDir, themeDir, sourceExt string) string {
	ext := strings.TrimPrefix(strings.ToLower(sourceExt), ".")
	formatFrom := FormatFromExtension(ext)
	formatTo := FormatCanvasTo(ext)
	now := time.Now().UTC().Format(time.RFC3339)

	var extra strings.Builder
	if formatFrom > 0 {
		fmt.Fprintf(&extra, "<m_nFormatFrom>%d</m_nFormatFrom>\n", formatFrom)
	}
	if ext == "csv" {
		extra.WriteString("<m_nCsvTxtEncoding>46</m_nCsvTxtEncoding>\n")
		extra.WriteString("<m_nCsvDelimiter>4</m_nCsvDelimiter>\n")
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<TaskQueueDataConvert xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
<m_sFileFrom>%s</m_sFileFrom>
<m_sFileTo>%s</m_sFileTo>
<m_nFormatTo>%d</m_nFormatTo>
%s<m_sFontDir>%s</m_sFontDir>
<m_sThemeDir>%s</m_sThemeDir>
<m_oTimestamp>%s</m_oTimestamp>
</TaskQueueDataConvert>
`, escapeXML(from), escapeXML(to), formatTo, extra.String(), escapeXML(fontDir), escapeXML(themeDir), now)
}

func buildReverseTaskXML(from, to, fontDir, themeDir, targetExt string, fromChanges bool) string {
	formatFrom := FormatCanvasTo(targetExt)
	formatTo := FormatFromExtension(targetExt)
	now := time.Now().UTC().Format(time.RFC3339)

	var extra strings.Builder
	if formatFrom > 0 {
		fmt.Fprintf(&extra, "<m_nFormatFrom>%d</m_nFormatFrom>\n", formatFrom)
	}
	if formatTo > 0 {
		fmt.Fprintf(&extra, "<m_nFormatTo>%d</m_nFormatTo>\n", formatTo)
	}
	if fromChanges {
		extra.WriteString("<m_bFromChanges>true</m_bFromChanges>\n")
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<TaskQueueDataConvert xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
<m_sFileFrom>%s</m_sFileFrom>
<m_sFileTo>%s</m_sFileTo>
%s<m_sFontDir>%s</m_sFontDir>
<m_sThemeDir>%s</m_sThemeDir>
<m_oTimestamp>%s</m_oTimestamp>
</TaskQueueDataConvert>
`, escapeXML(from), escapeXML(to), extra.String(), escapeXML(fontDir), escapeXML(themeDir), now)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
