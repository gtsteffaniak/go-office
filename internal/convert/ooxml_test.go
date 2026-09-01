package convert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOOXMLPlainTextSampleDocx(t *testing.T) {
	p := filepath.Join("..", "..", "sample-files", "sample.docx")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Skip("sample.docx missing")
	}
	text := OOXMLPlainText(raw)
	if text == "" {
		t.Fatal("word/document.xml had no w:t nodes")
	}
	t.Logf("sample.docx text: %q", text)
	if !strings.Contains(text, "Demonstration of DOCX") {
		t.Fatalf("playwright manifest text is not in sample.docx")
	}
}
