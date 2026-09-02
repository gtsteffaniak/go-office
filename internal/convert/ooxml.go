package convert

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ooxmlTextRe = regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)

// OOXMLPart returns a named entry from an OOXML zip (docx/xlsx/pptx).
func OOXMLPart(raw []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, io.ErrUnexpectedEOF
}

// OOXMLPlainText concatenates w:t nodes from word/document.xml, preserving
// paragraph breaks from w:p boundaries.
func OOXMLPlainText(raw []byte) string {
	part, err := OOXMLPart(raw, "word/document.xml")
	if err != nil {
		return ""
	}
	paragraphs := strings.Split(string(part), "</w:p>")
	lines := make([]string, 0, len(paragraphs))
	for _, para := range paragraphs {
		matches := ooxmlTextRe.FindAllSubmatch([]byte(para), -1)
		if len(matches) == 0 {
			continue
		}
		var b strings.Builder
		for _, m := range matches {
			if len(m) > 1 {
				b.Write(m[1])
			}
		}
		if b.Len() > 0 {
			lines = append(lines, b.String())
		}
	}
	return strings.Join(lines, "\n")
}

// WritePlainTextFromDocx extracts visible text from a docx and writes destPath.
// Used by tests and legacy tooling; txt persist goes through x2t txt export.
func WritePlainTextFromDocx(docxPath, destPath string) error {
	raw, err := os.ReadFile(docxPath)
	if err != nil {
		return err
	}
	body := normalizePlainTextBytes([]byte(OOXMLPlainText(raw)))
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destPath, body, 0o644)
}
