package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteRTFFromDocxPlainText writes a minimal RTF with plain ASCII body text extracted
// from a docx intermediate. Euro-Office x2t docx→rtf embeds AllFonts (~800KB) and encodes
// Latin text as \u escapes, which breaks substring checks on saved files.
func WriteRTFFromDocxPlainText(docxPath, destPath string) error {
	raw, err := os.ReadFile(docxPath)
	if err != nil {
		return err
	}
	text := OOXMLPlainText(raw)
	lines := strings.Split(text, "\n")
	var body strings.Builder
	for i, line := range lines {
		if i > 0 {
			body.WriteString("\\par\n")
		}
		body.WriteString(escapeRTFText(line))
	}
	out := fmt.Sprintf(
		"{\\rtf1\\ansi\\deff0{\\fonttbl{\\f0\\froman Times New Roman;}}\\f0\\fs24 %s}",
		body.String(),
	)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destPath, []byte(out), 0o644)
}

func escapeRTFText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	return s
}
