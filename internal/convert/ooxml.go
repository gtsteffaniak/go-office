package convert

import (
	"archive/zip"
	"bytes"
	"io"
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

// OOXMLPlainText concatenates w:t nodes from word/document.xml.
func OOXMLPlainText(raw []byte) string {
	part, err := OOXMLPart(raw, "word/document.xml")
	if err != nil {
		return ""
	}
	matches := ooxmlTextRe.FindAllSubmatch(part, -1)
	var b strings.Builder
	for _, m := range matches {
		if len(m) > 1 {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.Write(m[1])
		}
	}
	return b.String()
}
