package convert

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
)

// csvNativeSheetID is the worksheet id x2t assigns when importing a CSV.
// After an xlsx round-trip the spreadsheet editor uses ids like "1_4";
// apply_changes only honors the native id.
const csvNativeSheetID = "5"

func canonicalizeCSVChangeFiles(changesDir string) (int, error) {
	entries, err := os.ReadDir(changesDir)
	if err != nil {
		return 0, err
	}
	rewritten := 0
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".json") {
			continue
		}
		path := filepath.Join(changesDir, ent.Name())
		n, err := canonicalizeCSVChangesFile(path)
		if err != nil {
			return rewritten, err
		}
		rewritten += n
	}
	return rewritten, nil
}

func canonicalizeCSVChangesFile(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var blobs []string
	if err = json.Unmarshal(raw, &blobs); err != nil {
		return 0, err
	}
	changed := 0
	for i, blob := range blobs {
		next, ok := rewriteCSVChangeSheetID(blob)
		if !ok {
			continue
		}
		blobs[i] = next
		changed++
	}
	if changed == 0 {
		return 0, nil
	}
	out, err := json.Marshal(blobs)
	if err != nil {
		return 0, err
	}
	return changed, os.WriteFile(path, out, 0o644)
}

func rewriteCSVChangeSheetID(blob string) (string, bool) {
	nStr, b64, ok := strings.Cut(blob, ";")
	if !ok {
		return blob, false
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return blob, false
	}
	if n, err := strconv.Atoi(nStr); err == nil && n != len(raw) {
		return blob, false
	}
	next, ok := replaceExcelSheetID(raw, csvNativeSheetID)
	if !ok {
		return blob, false
	}
	return fmt.Sprintf("%d;%s", len(next), base64.StdEncoding.EncodeToString(next)), true
}

func replaceExcelSheetID(raw []byte, to string) ([]byte, bool) {
	const marker = "\x10\x01"
	idx := strings.Index(string(raw), marker)
	if idx < 0 {
		return nil, false
	}
	lenOff := idx + 2
	if lenOff+4 > len(raw) {
		return nil, false
	}
	strLen := int(binary.LittleEndian.Uint32(raw[lenOff : lenOff+4]))
	if strLen < 2 || strLen > 64 || strLen%2 != 0 {
		return nil, false
	}
	strStart := lenOff + 4
	strEnd := strStart + strLen
	if strEnd > len(raw) {
		return nil, false
	}
	old := utf16LEString(raw[strStart:strEnd])
	if old == to || !isExcelStyleSheetID(old) {
		return nil, false
	}
	newUTF16 := encodeUTF16LE(to)
	out := make([]byte, 0, len(raw)-strLen+len(newUTF16))
	out = append(out, raw[:lenOff]...)
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(newUTF16)))
	out = append(out, lenBuf[:]...)
	out = append(out, newUTF16...)
	out = append(out, raw[strEnd:]...)
	if len(out) >= 4 {
		binary.LittleEndian.PutUint32(out[:4], uint32(len(out)-4))
	}
	return out, true
}

func isExcelStyleSheetID(s string) bool {
	i := strings.IndexByte(s, '_')
	if i <= 0 || i == len(s)-1 {
		return false
	}
	return isDigits(s[:i]) && isDigits(s[i+1:])
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func utf16LEString(b []byte) string {
	if len(b)%2 != 0 {
		return ""
	}
	u := make([]uint16, len(b)/2)
	for i := 0; i < len(u); i++ {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u))
}

func encodeUTF16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, len(u)*2)
	for i, r := range u {
		binary.LittleEndian.PutUint16(out[i*2:], r)
	}
	return out
}
