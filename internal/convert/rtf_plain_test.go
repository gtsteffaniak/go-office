package convert

import (
	"strings"
	"testing"
)

func TestRTFPlainTextDecodesUCUnicode(t *testing.T) {
	raw := "{\\uc1\\u83*\\u117*\\u98*\\u116*\\u105*\\u116*\\u108*\\u101*}"
	got := RTFPlainText([]byte(raw))
	if got != "Subtitle" {
		t.Fatalf("got %q want %q", got, "Subtitle")
	}
}

func TestRTFPlainTextDecodesBrief(t *testing.T) {
	raw := "{\\uc1\\u66*\\u82*\\u73*\\u69*\\u70*}"
	got := RTFPlainText([]byte(raw))
	if got != "BRIEF" {
		t.Fatalf("got %q want %q", got, "BRIEF")
	}
}

func TestRTFPlainTextPlainASCII(t *testing.T) {
	raw := "{\\rtf1\\ansi SYSTEM BRIEF}"
	got := RTFPlainText([]byte(raw))
	if !strings.Contains(got, "SYSTEM BRIEF") {
		t.Fatalf("got %q", got)
	}
}
