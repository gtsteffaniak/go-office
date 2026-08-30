package convert_test

import (
	"testing"

	"github.com/quantumx-apps/go-office/internal/convert"
)

func TestFormatCanvasTo(t *testing.T) {
	cases := []struct {
		ext  string
		want int
	}{
		{"docx", convert.FormatCanvasWord},
		{"csv", convert.FormatCanvasSpreadsheet},
		{"xlsx", convert.FormatCanvasSpreadsheet},
		{"pptx", convert.FormatCanvasPresentation},
		{"pdf", convert.FormatCanvasPDF},
	}
	for _, tc := range cases {
		if got := convert.FormatCanvasTo(tc.ext); got != tc.want {
			t.Fatalf("FormatCanvasTo(%q) = %#x, want %#x", tc.ext, got, tc.want)
		}
	}
}

func TestIsBrowserEditorFormat(t *testing.T) {
	if !convert.IsBrowserEditorFormat("pdf") {
		t.Fatal("pdf should be browser editor format")
	}
	if convert.IsBrowserEditorFormat("docx") {
		t.Fatal("docx should not be browser editor format")
	}
}

func TestFormatFromExtension(t *testing.T) {
	if got := convert.FormatFromExtension("csv"); got != 0x0104 {
		t.Fatalf("csv = %#x", got)
	}
	if got := convert.FormatFromExtension("dotx"); got != 0x004c {
		t.Fatalf("dotx = %#x", got)
	}
}
