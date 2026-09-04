package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// RTFPlainText extracts human-readable text from RTF, decoding \u and \' escapes.
func RTFPlainText(raw []byte) string {
	s := string(raw)
	var out strings.Builder
	i := 0
	ucSkip := 1
	for i < len(s) {
		if i+1 < len(s) && s[i] == '\\' {
			if i+3 < len(s) && s[i+1] == '\'' {
				hex := s[i+2 : i+4]
				if b, err := strconv.ParseUint(hex, 16, 8); err == nil {
					out.WriteByte(byte(b))
					i += 4
					continue
				}
			}
			if i+4 < len(s) && s[i+1] == 'p' && s[i+2] == 'a' && s[i+3] == 'r' {
				out.WriteByte('\n')
				i += 4
				if i < len(s) && s[i] == ' ' {
					i++
				}
				continue
			}
			if i+3 < len(s) && s[i+1] == 'u' && s[i+2] == 'c' {
				j := i + 3
				start := j
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
				if j > start {
					if n, err := strconv.Atoi(s[start:j]); err == nil {
						ucSkip = n
					}
					if j < len(s) && s[j] == ' ' {
						j++
					}
					i = j
					continue
				}
			}
			if s[i+1] == 'u' {
				j := i + 2
				neg := false
				if j < len(s) && s[j] == '-' {
					neg = true
					j++
				}
				start := j
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
				if j > start {
					n, _ := strconv.Atoi(s[start:j])
					if neg {
						n = 65536 + n
					}
					out.WriteRune(rune(n))
					if j < len(s) && s[j] == '?' {
						j++
					}
					for skipped := 0; skipped < ucSkip && j < len(s); skipped++ {
						j++
					}
					i = j
					continue
				}
			}
			j := i + 1
			if j < len(s) && s[j] == '*' {
				j++
			}
			for j < len(s) && ((s[j] >= 'a' && s[j] <= 'z') || (s[j] >= 'A' && s[j] <= 'Z')) {
				j++
			}
			if j < len(s) && s[j] == '-' {
				j++
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
			} else if j < len(s) && s[j] >= '0' && s[j] <= '9' {
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
			}
			if j < len(s) && s[j] == ' ' {
				j++
			}
			i = j
			continue
		}
		if s[i] == '{' || s[i] == '}' {
			i++
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func escapeRTFText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	return s
}
