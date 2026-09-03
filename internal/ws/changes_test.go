package ws

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendChangesUsesChanges0JSON(t *testing.T) {
	dir := t.TempDir()
	if _, err := appendChanges(dir, []string{"c1", "c2"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "changes", changesFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "c1" || got[1] != "c2" {
		t.Fatalf("changes0.json = %#v", got)
	}

	if _, appendErr := appendChanges(dir, []string{"c3"}); appendErr != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected merged changes, got %#v", got)
	}
}

func TestParseChangesFromStringPayload(t *testing.T) {
	msg := map[string]any{
		"changes": `["a","b"]`,
	}
	got := parseChanges(msg)
	if len(got) != 2 || got[0] != "a" {
		t.Fatalf("parseChanges = %#v", got)
	}
}

func TestParseChangesExcelEscapedJSON(t *testing.T) {
	// Spreadsheet saveChanges sends the array as a JSON string, not []any.
	msg := map[string]any{
		"changes": `["14;CgAAAAFiAAAA/wAAAAA=","128;fAAAAAFkBwAABXIAAAAAAAE="]`,
	}
	got := parseChanges(msg)
	if len(got) != 2 {
		t.Fatalf("excel string changes = %#v", got)
	}
	if _, err := appendChanges(t.TempDir(), got); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeChangeBlobUTF16(t *testing.T) {
	got := decodeChangeBlob("64;AgAAADEA//8BAIbRyCG/xwQApwAAAAEAAAAAAAAAAAAAAAEAAAAAAAAA9v///w4AAAA5AC4AMwAuADQALgAwAA==")
	if !strings.Contains(got, "9.3.4.0") {
		t.Fatalf("preview = %q", got)
	}
}
