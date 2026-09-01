package convert

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	assetDir      string
	binDir        string
	fontDir       string
	saveFontDir   string
	themeDir      string
	webAllFonts   string // sdkjs bundle for browser only
	seedAllFonts  []byte // converter/bin/AllFonts.js frozen at startup
	fontSelection []byte // converter/bin/font_selection.bin frozen at startup
	limit         chan struct{}
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
	if err := fixDoctRendererConfig(binDir); err != nil {
		slog.Warn("x2t doctrenderer config", "err", err)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 1
	}
	fontDir := filepath.Join(opts.AssetDir, "core-fonts")
	if st, err := os.Stat(fontDir); err != nil || !st.IsDir() {
		fontDir = filepath.Join(opts.AssetDir, "fonts")
	}
	fontSelPath := filepath.Join(binDir, "font_selection.bin")
	rawAllFonts, err := os.ReadFile(filepath.Join(binDir, "AllFonts.js"))
	if err != nil {
		return nil, fmt.Errorf("convert: read AllFonts.js: %w", err)
	}
	ensureStaleFontPrefixAlias(rawAllFonts, opts.AssetDir)
	seedAllFonts := rewriteAllFontsPaths(rawAllFonts, opts.AssetDir)
	fontSel, _ := os.ReadFile(fontSelPath)
	return &Converter{
		assetDir:      opts.AssetDir,
		binDir:        binDir,
		fontDir:       fontDir,
		saveFontDir:   binDir,
		themeDir:      filepath.Join(opts.AssetDir, "sdkjs", "slide", "themes"),
		webAllFonts:   filepath.Join(opts.AssetDir, "sdkjs", "common", "AllFonts.js"),
		seedAllFonts:  seedAllFonts,
		fontSelection: fontSel,
		limit:         make(chan struct{}, limit),
	}, nil
}

// ToEditorBin converts sourcePath into outDir/Editor.bin.
func (c *Converter) ToEditorBin(ctx context.Context, sourcePath, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	return withCacheDirLock(outDir, func() error {
		return c.toEditorBin(ctx, sourcePath, outDir)
	})
}

func (c *Converter) toEditorBin(ctx context.Context, sourcePath, outDir string) error {
	outFile := filepath.Join(outDir, "Editor.bin")
	partFile := filepath.Join(outDir, "Editor.bin.part")
	convertPath := sourcePath
	srcHash, err := fileSHA256(sourcePath)
	if err != nil {
		return err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(sourcePath)), ".")
	if csvNeedsXlsxBridge(ext) {
		raw, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		norm := normalizeCSVBytes(raw)
		srcHash = sha256Bytes(norm)
		if !bytes.Equal(raw, norm) {
			tmp, err := os.CreateTemp("", "go-office-csv-*.csv")
			if err != nil {
				return err
			}
			convertPath = tmp.Name()
			if _, err := tmp.Write(norm); err != nil {
				tmp.Close()
				_ = os.Remove(convertPath)
				return err
			}
			if err := tmp.Close(); err != nil {
				_ = os.Remove(convertPath)
				return err
			}
			defer os.Remove(convertPath)
		}
	}
	if editorBinReusable(outDir, srcHash) {
		return ensureDocumentFonts(c, outDir)
	}
	_ = os.Remove(outFile)
	_ = os.Remove(partFile)
	_ = os.Remove(sourceHashPath(outDir))

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

	runDir, err := c.prepareX2TRunDir("")
	if err != nil {
		return err
	}
	defer os.RemoveAll(runDir)

	allFontsPath := filepath.Join(runDir, "AllFonts.js")
	workDir := filepath.Join(runDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	xml := buildTaskXML(convertPath, partFile, c.fontDir, c.themeDir, filepath.Ext(sourcePath), allFontsPath, workDir)
	if _, err := taskFile.WriteString(xml); err != nil {
		taskFile.Close()
		return err
	}
	if err := taskFile.Close(); err != nil {
		return err
	}

	out, err := c.runX2t(ctx, taskPath, runDir)
	if err != nil {
		return fmt.Errorf("convert: x2t: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if st, err := os.Stat(partFile); err != nil || st.Size() == 0 {
		_ = os.Remove(partFile)
		return fmt.Errorf("convert: x2t produced no output")
	}
	if err := os.Rename(partFile, outFile); err != nil {
		_ = os.Remove(partFile)
		return err
	}
	if err := snapshotFontArtifacts(runDir, outDir); err != nil {
		return err
	}
	if err := ensureDocumentFonts(c, outDir); err != nil {
		return err
	}
	if err := writeSourceHash(outDir, srcHash); err != nil {
		return err
	}
	return nil
}

// FromEditorBin converts cacheDir/Editor.bin to destPath (e.g. saved.docx).
func (c *Converter) FromEditorBin(ctx context.Context, cacheDir, destPath, targetExt string) error {
	return withCacheDirLock(cacheDir, func() error {
		editorBin := filepath.Join(cacheDir, "Editor.bin")
		if st, err := os.Stat(editorBin); err != nil || st.Size() == 0 {
			return fmt.Errorf("convert: Editor.bin missing in %s", cacheDir)
		}
		return c.fromEditor(ctx, editorBin, destPath, targetExt, false)
	})
}

// SaveChanges applies cacheDir/changes/*.json on top of Editor.bin and writes destPath.
func (c *Converter) SaveChanges(ctx context.Context, cacheDir, destPath, targetExt string) error {
	return withCacheDirLock(cacheDir, func() error {
		return c.saveChanges(ctx, cacheDir, destPath, targetExt)
	})
}

func (c *Converter) saveChanges(ctx context.Context, cacheDir, destPath, targetExt string) error {
	editorBin := filepath.Join(cacheDir, "Editor.bin")
	if st, err := os.Stat(editorBin); err != nil || st.Size() == 0 {
		return fmt.Errorf("convert: Editor.bin missing in %s", cacheDir)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	entries, err := os.ReadDir(changesDir)
	if err != nil || len(entries) == 0 {
		return c.fromEditor(ctx, editorBin, destPath, targetExt, false)
	}
	ext := strings.TrimPrefix(strings.ToLower(targetExt), ".")
	slog.Debug("save changes",
		"cache", cacheDir,
		"dest", destPath,
		"ext", ext,
		"changeFiles", len(entries),
		"xlsxBridge", csvNeedsXlsxBridge(ext),
	)
	// x2t's Editor.bin → CSV path (xlst_bin2csv) never calls apply_changes.
	// Spreadsheet coauthoring patches are applied on the XLSX path, then we
	// convert that workbook to CSV.
	if csvNeedsXlsxBridge(ext) {
		n, err := canonicalizeCSVChangeFiles(changesDir)
		if err != nil {
			return err
		}
		if n > 0 {
			slog.Debug("csv change sheet id remapped", "blobs", n, "to", csvNativeSheetID)
		}
		xlsxPath := filepath.Join(cacheDir, "changes-applied.xlsx")
		if err := c.fromEditor(ctx, editorBin, xlsxPath, "xlsx", true); err != nil {
			return err
		}
		xlsxSize := int64(0)
		if st, err := os.Stat(xlsxPath); err == nil {
			xlsxSize = st.Size()
		}
		slog.Debug("xlsx after apply_changes", "path", xlsxPath, "bytes", xlsxSize)
		if err := c.convertOffice(ctx, xlsxPath, destPath, "xlsx", ext, cacheDir); err != nil {
			return err
		}
		stripped, err := rewriteNormalizedCSV(destPath)
		if err != nil {
			return err
		}
		slog.Debug("csv persist normalized", "path", destPath, "stripped", stripped)
		return nil
	}
	return c.fromEditor(ctx, editorBin, destPath, targetExt, true)
}

func csvNeedsXlsxBridge(ext string) bool {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "csv", "tsv", "scsv":
		return true
	default:
		return false
	}
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

	cacheDir := filepath.Dir(editorBin)
	runDir, err := c.prepareX2TRunDir(cacheDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(runDir)

	taskFile, err := os.CreateTemp("", "go-office-x2t-save-*.xml")
	if err != nil {
		return err
	}
	taskPath := taskFile.Name()
	defer os.Remove(taskPath)

	ext := strings.TrimPrefix(strings.ToLower(targetExt), ".")
	allFontsPath := filepath.Join(runDir, "AllFonts.js")
	workDir := filepath.Join(runDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	c.logFontSources("reverse", cacheDir, allFontsPath)
	xml := buildReverseTaskXML(editorBin, destPath, c.fontDir, c.themeDir, allFontsPath, ext, fromChanges, workDir)
	if _, err := taskFile.WriteString(xml); err != nil {
		taskFile.Close()
		return err
	}
	if err := taskFile.Close(); err != nil {
		return err
	}

	out, err := c.runX2t(ctx, taskPath, runDir)
	if err != nil {
		return fmt.Errorf("convert: reverse x2t: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if st, err := os.Stat(destPath); err != nil || st.Size() == 0 {
		return fmt.Errorf("convert: reverse x2t produced no output")
	}
	if err := snapshotFontArtifacts(runDir, cacheDir); err != nil {
		return err
	}
	return nil
}

// prepareX2TRunDir creates an isolated cwd for DoctRenderer so concurrent x2t
// processes never read or write shared AllFonts.js under converter/bin.
// cacheDir is the per-document cache (assets/cache/<key>/); when empty, fonts
// are seeded from the converter/bin/AllFonts.js snapshot frozen at startup.
func (c *Converter) prepareX2TRunDir(cacheDir string) (string, error) {
	runRoot := filepath.Join(c.binDir, ".run")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		return "", err
	}
	runDir, err := os.MkdirTemp(runRoot, "x2t-*")
	if err != nil {
		return "", err
	}
	if _, err := c.stageReverseFonts(runDir, cacheDir); err != nil {
		os.RemoveAll(runDir)
		return "", err
	}
	if err := writeRunDoctRendererConfig(runDir, c.assetDir); err != nil {
		os.RemoveAll(runDir)
		return "", err
	}
	// DoctRenderer resolves paths from the x2t binary location (/proc/self/exe).
	// Symlinks still resolve to converter/bin, so copy the small x2t stub (~64KiB).
	if err := copyExecutable(filepath.Join(c.binDir, "x2t"), filepath.Join(runDir, "x2t")); err != nil {
		os.RemoveAll(runDir)
		return "", err
	}
	return runDir, nil
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// stageReverseFonts copies x2t font metadata into an isolated work dir so concurrent
// reverse conversions do not race on shared files under converter/bin.
func (c *Converter) stageReverseFonts(workDir, cacheDir string) (string, error) {
	allFontsPath := filepath.Join(workDir, "AllFonts.js")
	allFontsData := c.seedAllFonts
	if cacheDir != "" {
		if raw, err := os.ReadFile(filepath.Join(cacheDir, "AllFonts.js")); err == nil && len(raw) > 0 {
			allFontsData = raw
		}
	}
	if len(allFontsData) == 0 {
		return "", fmt.Errorf("convert: no AllFonts.js seed")
	}
	if err := os.WriteFile(allFontsPath, allFontsData, 0o644); err != nil {
		return "", err
	}
	selData := c.fontSelection
	if cacheDir != "" {
		if raw, err := os.ReadFile(filepath.Join(cacheDir, "font_selection.bin")); err == nil && len(raw) > 0 {
			selData = raw
		}
	}
	if len(selData) > 0 {
		if err := os.WriteFile(filepath.Join(workDir, "font_selection.bin"), selData, 0o644); err != nil {
			return "", err
		}
	}
	return allFontsPath, nil
}

func ensureDocumentFonts(c *Converter, cacheDir string) error {
	allFontsPath := filepath.Join(cacheDir, "AllFonts.js")
	if st, err := os.Stat(allFontsPath); err != nil || st.Size() == 0 {
		if len(c.seedAllFonts) == 0 {
			return fmt.Errorf("convert: no AllFonts.js seed")
		}
		if err := os.WriteFile(allFontsPath, c.seedAllFonts, 0o644); err != nil {
			return err
		}
	}
	selPath := filepath.Join(cacheDir, "font_selection.bin")
	if st, err := os.Stat(selPath); err == nil && st.Size() > 0 {
		return nil
	}
	selRaw, err := c.readBinFontSelection()
	if err != nil || len(selRaw) == 0 {
		return nil
	}
	return os.WriteFile(selPath, selRaw, 0o644)
}

func (c *Converter) logFontSources(kind, cacheDir, runAllFonts string) {
	if cacheDir == "" {
		return
	}
	src := "seed"
	if cacheDir != "" {
		if st, err := os.Stat(filepath.Join(cacheDir, "AllFonts.js")); err == nil && st.Size() > 0 {
			src = "cache"
		}
	}
	slog.Debug("x2t font isolation",
		"kind", kind,
		"cache", cacheDir,
		"allFontsSource", src,
		"runAllFonts", runAllFonts,
	)
}

func (c *Converter) readBinFontSelection() ([]byte, error) {
	if len(c.fontSelection) == 0 {
		return nil, os.ErrNotExist
	}
	out := make([]byte, len(c.fontSelection))
	copy(out, c.fontSelection)
	return out, nil
}

func snapshotFontArtifacts(fromDir, cacheDir string) error {
	for _, name := range []string{"AllFonts.js", "font_selection.bin"} {
		src := filepath.Join(fromDir, name)
		st, err := os.Stat(src)
		if err != nil || st.Size() == 0 {
			continue
		}
		if err := copyFile(src, filepath.Join(cacheDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (c *Converter) convertOffice(ctx context.Context, srcPath, destPath, fromExt, toExt, cacheDir string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	select {
	case c.limit <- struct{}{}:
		defer func() { <-c.limit }()
	case <-ctx.Done():
		return ctx.Err()
	}

	runDir, err := c.prepareX2TRunDir(cacheDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(runDir)

	taskFile, err := os.CreateTemp("", "go-office-x2t-fmt-*.xml")
	if err != nil {
		return err
	}
	taskPath := taskFile.Name()
	defer os.Remove(taskPath)

	allFontsPath := filepath.Join(runDir, "AllFonts.js")
	workDir := filepath.Join(runDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	c.logFontSources("office", cacheDir, allFontsPath)
	xml := buildOfficeToOfficeXML(srcPath, destPath, c.fontDir, c.themeDir, allFontsPath, fromExt, toExt, workDir)
	if _, err := taskFile.WriteString(xml); err != nil {
		taskFile.Close()
		return err
	}
	if err := taskFile.Close(); err != nil {
		return err
	}
	out, err := c.runX2t(ctx, taskPath, runDir)
	if err != nil {
		return fmt.Errorf("convert: x2t %s→%s: %w: %s", fromExt, toExt, err, strings.TrimSpace(string(out)))
	}
	if st, err := os.Stat(destPath); err != nil || st.Size() == 0 {
		return fmt.Errorf("convert: x2t %s→%s produced no output", fromExt, toExt)
	}
	if cacheDir != "" {
		if err := snapshotFontArtifacts(runDir, cacheDir); err != nil {
			return err
		}
	}
	return nil
}

// runX2t executes x2t via /bin/sh so chmod and LD_LIBRARY_PATH match Nextcloud's approach.
// When isolatedDir is set, x2t runs with that cwd and a private DoctRenderer.config so
// concurrent saves do not share converter/bin/AllFonts.js.
func (c *Converter) runX2t(ctx context.Context, taskPath string, isolatedDir string) ([]byte, error) {
	dir := c.binDir
	if isolatedDir != "" {
		dir = isolatedDir
	}
	// Always invoke ./x2t relative to dir. Isolated runs use a copied binary so DoctRenderer
	// picks up the per-run config beside the binary, not converter/bin/AllFonts.js.
	script := fmt.Sprintf(
		"chmod +x ./x2t 2>/dev/null; LD_LIBRARY_PATH=%s exec ./x2t %s",
		shellQuote(c.binDir),
		shellQuote(taskPath),
	)
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
	cmd.Dir = dir
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

func buildTaskXML(from, to, fontDir, themeDir, sourceExt, allFonts, tempDir string) string {
	ext := strings.TrimPrefix(strings.ToLower(sourceExt), ".")
	formatFrom := FormatFromExtension(ext)
	formatTo := FormatCanvasTo(ext)
	now := time.Now().UTC().Format(time.RFC3339)

	var extra strings.Builder
	if formatFrom > 0 {
		fmt.Fprintf(&extra, "<m_nFormatFrom>%d</m_nFormatFrom>\n", formatFrom)
	}
	switch ext {
	case "csv", "tsv", "scsv", "txt":
		extra.WriteString("<m_nCsvTxtEncoding>46</m_nCsvTxtEncoding>\n")
	}
	if ext == "csv" {
		extra.WriteString("<m_nCsvDelimiter>4</m_nCsvDelimiter>\n")
	}
	if allFonts != "" {
		fmt.Fprintf(&extra, "<m_sAllFontsPath>%s</m_sAllFontsPath>\n", escapeXML(allFonts))
	}
	if tempDir != "" {
		fmt.Fprintf(&extra, "<m_sTempDir>%s</m_sTempDir>\n", escapeXML(tempDir))
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

func buildReverseTaskXML(from, to, fontDir, themeDir, allFonts, targetExt string, fromChanges bool, tempDir string) string {
	formatFrom := FormatCanvasTo(targetExt)
	formatTo := FormatFromExtension(targetExt)
	now := time.Now().UTC().Format(time.RFC3339)

	var extra strings.Builder
	if !fromChanges && formatFrom > 0 {
		fmt.Fprintf(&extra, "<m_nFormatFrom>%d</m_nFormatFrom>\n", formatFrom)
	}
	if formatTo > 0 {
		fmt.Fprintf(&extra, "<m_nFormatTo>%d</m_nFormatTo>\n", formatTo)
	}
	if fromChanges {
		extra.WriteString("<m_bFromChanges>true</m_bFromChanges>\n")
		extra.WriteString("<m_bIsNoBase64>false</m_bIsNoBase64>\n")
		extra.WriteString("<m_nDoctParams>1</m_nDoctParams>\n")
		if tempDir != "" {
			fmt.Fprintf(&extra, "<m_sTempDir>%s</m_sTempDir>\n", escapeXML(tempDir))
		}
		if isSpreadsheetExt(targetExt) {
			extra.WriteString("<m_sJsonParams>{&quot;spreadsheetLayout&quot;:{&quot;fitToWidth&quot;:1,&quot;fitToHeight&quot;:1}}</m_sJsonParams>\n")
		}
	} else {
		extra.WriteString("<m_bFromChanges>false</m_bFromChanges>\n")
	}
	extra.WriteString("<m_bDontSaveAdditional>true</m_bDontSaveAdditional>\n")
	fmt.Fprintf(&extra, "<m_sAllFontsPath>%s</m_sAllFontsPath>\n", escapeXML(allFonts))
	extra.WriteString("<m_nCsvTxtEncoding>46</m_nCsvTxtEncoding>\n")
	switch strings.TrimPrefix(strings.ToLower(targetExt), ".") {
	case "tsv":
		extra.WriteString("<m_nCsvDelimiter>1</m_nCsvDelimiter>\n")
	case "scsv":
		extra.WriteString("<m_nCsvDelimiter>2</m_nCsvDelimiter>\n")
	case "csv", "xlsx", "xls", "ods":
		extra.WriteString("<m_nCsvDelimiter>4</m_nCsvDelimiter>\n")
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

func buildOfficeToOfficeXML(from, to, fontDir, themeDir, allFonts, fromExt, toExt, tempDir string) string {
	fromExt = strings.TrimPrefix(strings.ToLower(fromExt), ".")
	toExt = strings.TrimPrefix(strings.ToLower(toExt), ".")
	formatFrom := FormatFromExtension(fromExt)
	formatTo := FormatFromExtension(toExt)
	now := time.Now().UTC().Format(time.RFC3339)
	var extra strings.Builder
	if formatFrom > 0 {
		fmt.Fprintf(&extra, "<m_nFormatFrom>%d</m_nFormatFrom>\n", formatFrom)
	}
	if formatTo > 0 {
		fmt.Fprintf(&extra, "<m_nFormatTo>%d</m_nFormatTo>\n", formatTo)
	}
	if allFonts != "" {
		fmt.Fprintf(&extra, "<m_sAllFontsPath>%s</m_sAllFontsPath>\n", escapeXML(allFonts))
	}
	if tempDir != "" {
		fmt.Fprintf(&extra, "<m_sTempDir>%s</m_sTempDir>\n", escapeXML(tempDir))
	}
	extra.WriteString("<m_nCsvTxtEncoding>46</m_nCsvTxtEncoding>\n")
	extra.WriteString("<m_nCsvDelimiter>4</m_nCsvDelimiter>\n")
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

func isSpreadsheetExt(ext string) bool {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "csv", "tsv", "scsv", "xlsx", "xls", "xlsm", "xlsb", "ods", "ots":
		return true
	default:
		return false
	}
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
