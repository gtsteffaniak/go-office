package convert

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOOXMLPlainTextPreservesParagraphs(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<w:document><w:body>
<w:p><w:r><w:t>Line one</w:t></w:r></w:p>
<w:p><w:r><w:t>Line two</w:t></w:r></w:p>
</w:body></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	got := OOXMLPlainText(buf.Bytes())
	want := "Line one\nLine two"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWritePlainTextFromDocx(t *testing.T) {
	p := filepath.Join("..", "..", "sample-files", "sample.docx")
	if _, err := os.Stat(p); err != nil {
		t.Skip("sample.docx missing")
	}
	dir := t.TempDir()
	docx := filepath.Join(dir, "applied.docx")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(docx, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "saved.txt")
	if err = WritePlainTextFromDocx(docx, out); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Demonstration of DOCX") {
		t.Fatalf("unexpected txt: %q", body)
	}
}
