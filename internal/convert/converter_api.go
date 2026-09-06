package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Thumbnail configures raster output sizing for image conversions.
type Thumbnail struct {
	Width  int
	Height int
	Aspect int  // 0 stretch, 1 keep aspect, 2 page metrics at 96dpi (default)
	First  bool // first page only; false → zip of pages
}

// ConvertRequest is a direct file conversion (POST /converter).
type ConvertRequest struct {
	SourcePath  string
	DestPath    string
	FileType    string
	OutputType  string
	Thumbnail   *Thumbnail
	LowPriority bool // demo thumbnails; admitted after editor open/save waiters
}

// ConvertOffice runs x2t office-to-office conversion (e.g. csv → ods, xlsx → ods).
func (c *Converter) ConvertOffice(ctx context.Context, srcPath, destPath, fromExt, toExt string) error {
	if c == nil {
		return fmt.Errorf("convert: converter is nil")
	}
	if srcPath == "" || destPath == "" {
		return fmt.Errorf("convert: source and dest paths are required")
	}
	return c.convertOffice(ctx, srcPath, destPath, fromExt, toExt, "")
}

// ConvertFile runs x2t to convert SourcePath to DestPath.
func (c *Converter) ConvertFile(ctx context.Context, req ConvertRequest) error {
	if c == nil {
		return fmt.Errorf("convert: converter is nil")
	}
	if req.SourcePath == "" || req.DestPath == "" {
		return fmt.Errorf("convert: source and dest paths are required")
	}
	fromExt := strings.TrimPrefix(strings.ToLower(req.FileType), ".")
	if fromExt == "" {
		fromExt = strings.TrimPrefix(strings.ToLower(filepath.Ext(req.SourcePath)), ".")
	}
	toExt := strings.TrimPrefix(strings.ToLower(req.OutputType), ".")
	if toExt == "" {
		return fmt.Errorf("convert: output type is required")
	}
	formatTo := FormatOutputExtension(toExt)
	if formatTo == 0 {
		return fmt.Errorf("convert: unsupported output type %q", req.OutputType)
	}
	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return err
	}

	var release func()
	var err error
	if req.LowPriority {
		release, err = c.acquireConvertSlotLow(ctx)
	} else {
		release, err = c.acquireConvertSlot(ctx)
	}
	if err != nil {
		return err
	}
	defer release()

	runDir, err := c.prepareX2TRunDir("")
	if err != nil {
		return err
	}
	defer os.RemoveAll(runDir)

	taskFile, err := os.CreateTemp("", "go-office-convert-*.xml")
	if err != nil {
		return err
	}
	taskPath := taskFile.Name()
	defer os.Remove(taskPath)

	allFontsPath := filepath.Join(runDir, "AllFonts.js")
	workDir := filepath.Join(runDir, "work")
	if err = os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	xml := buildConvertTaskXML(req.SourcePath, req.DestPath, c.fontDir, c.themeDir, fromExt, toExt, formatTo, req.Thumbnail, allFontsPath, workDir)
	if _, err = taskFile.WriteString(xml); err != nil {
		taskFile.Close()
		return err
	}
	if err = taskFile.Close(); err != nil {
		return err
	}

	out, err := c.runX2t(ctx, taskPath, runDir)
	if err != nil {
		return fmt.Errorf("convert: x2t: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if st, err := os.Stat(req.DestPath); err != nil || st.Size() == 0 {
		return fmt.Errorf("convert: x2t produced no output")
	}
	return nil
}

func buildConvertTaskXML(from, to, fontDir, themeDir, fromExt, toExt string, formatTo int, thumb *Thumbnail, allFonts, tempDir string) string {
	formatFrom := FormatFromExtension(fromExt)
	now := time.Now().UTC().Format(time.RFC3339)

	var extra strings.Builder
	if formatFrom > 0 {
		fmt.Fprintf(&extra, "<m_nFormatFrom>%d</m_nFormatFrom>\n", formatFrom)
	}
	if formatTo > 0 {
		fmt.Fprintf(&extra, "<m_nFormatTo>%d</m_nFormatTo>\n", formatTo)
	}
	switch strings.TrimPrefix(strings.ToLower(fromExt), ".") {
	case "csv", "tsv", "scsv", "txt":
		extra.WriteString("<m_nCsvTxtEncoding>46</m_nCsvTxtEncoding>\n")
	}
	if strings.TrimPrefix(strings.ToLower(fromExt), ".") == "csv" {
		extra.WriteString("<m_nCsvDelimiter>4</m_nCsvDelimiter>\n")
	}
	if allFonts != "" {
		fmt.Fprintf(&extra, "<m_sAllFontsPath>%s</m_sAllFontsPath>\n", escapeXML(allFonts))
	}
	if tempDir != "" {
		fmt.Fprintf(&extra, "<m_sTempDir>%s</m_sTempDir>\n", escapeXML(tempDir))
	}
	if thumb != nil || IsImageOutput(toExt) {
		if jsonParams := thumbnailJSONParams(thumb, toExt); jsonParams != "" {
			fmt.Fprintf(&extra, "<m_sJsonParams>%s</m_sJsonParams>\n", escapeXML(jsonParams))
		}
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

func thumbnailJSONParams(thumb *Thumbnail, outputExt string) string {
	if thumb == nil && !IsImageOutput(outputExt) {
		return ""
	}
	t := map[string]any{}
	if thumb != nil {
		tm := map[string]any{}
		if thumb.Width > 0 {
			tm["width"] = thumb.Width
		}
		if thumb.Height > 0 {
			tm["height"] = thumb.Height
		}
		if thumb.Aspect >= 0 {
			tm["aspect"] = thumb.Aspect
		} else {
			tm["aspect"] = 2
		}
		tm["first"] = thumb.First
		t["thumbnail"] = tm
	} else if IsImageOutput(outputExt) {
		t["thumbnail"] = map[string]any{"aspect": 2, "first": true}
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return ""
	}
	return string(raw)
}
