package convert

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

func stripUTF8BOM(raw []byte) []byte {
	return bytes.TrimPrefix(raw, utf8BOM)
}

// normalizeCSVBytes makes x2t re-import a saved CSV as a native sheet (id "5")
// rather than an Excel workbook (sheet id "1_4"), so later saveChanges blobs apply.
func normalizeCSVBytes(raw []byte) []byte {
	raw = stripUTF8BOM(raw)
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	raw = bytes.ReplaceAll(raw, []byte("\r"), []byte("\n"))
	return raw
}

func rewriteNormalizedCSV(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	norm := normalizeCSVBytes(raw)
	if bytes.Equal(raw, norm) {
		return false, nil
	}
	return true, os.WriteFile(path, norm, 0o644)
}

func sha256Bytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
