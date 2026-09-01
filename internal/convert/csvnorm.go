package convert

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
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

func normalizePlainTextBytes(raw []byte) []byte {
	raw = stripUTF8BOM(raw)
	raw = bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	raw = bytes.ReplaceAll(raw, []byte("\r"), []byte("\n"))
	return raw
}

// NormalizePersistedOutput normalizes flat-text save output before writing to storage.
func NormalizePersistedOutput(ext string, raw []byte) []byte {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "csv", "tsv", "scsv":
		return normalizeCSVBytes(raw)
	case "txt":
		return normalizePlainTextBytes(raw)
	default:
		return raw
	}
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

func rewriteNormalizedTxt(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	norm := normalizePlainTextBytes(raw)
	if bytes.Equal(raw, norm) {
		return false, nil
	}
	return true, os.WriteFile(path, norm, 0o644)
}

func sha256Bytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
