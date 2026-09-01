package convert

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRewriteCSVChangeSheetID(t *testing.T) {
	const sheet14 = "82;TgAAAAEeEAEGAAAAMQBfADQAAQAAAAABAAAAAAAAAAEAAAAAKgAAAAACAQECAAIJAx0AAAAAAAEJAQoAAAAAAAEAAgIEAwIAAgADAQQBBQEGAQ=="
	const sheet5 = "78;SgAAAAEeEAECAAAANQABAAAAAAEAAAAAAAAAAQAAAAAqAAAAAAIBAQIAAgkDHQAAAAAAAQkBCgAAAAAAAQACAgMDAgACAAMBBAEFAQYB"

	got, ok := rewriteCSVChangeSheetID(sheet14)
	if !ok {
		t.Fatal("expected 1_4 blob to be rewritten to sheet 5")
	}
	_, b64, _ := strings.Cut(got, ";")
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "1\x00_\x004\x00") {
		t.Fatalf("sheet 1_4 still present: %q", got)
	}
	if !strings.Contains(string(raw), "5\x00") {
		t.Fatalf("sheet 5 missing after rewrite: %q", got)
	}

	if _, ok := rewriteCSVChangeSheetID(sheet5); ok {
		t.Fatal("native sheet 5 blob should be left alone")
	}
}
