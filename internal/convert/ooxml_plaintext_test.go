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

func writeTestDocxZip(t *testing.T, documentXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte(documentXML)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestOOXMLPlainTextDecodesEntities(t *testing.T) {
	raw := writeTestDocxZip(t, `<w:document><w:body>
<w:p><w:r><w:t>SYSTEM BRIEF &amp; DAILY LOG</w:t></w:r></w:p>
<w:p><w:r><w:t>&gt; Reminder</w:t></w:r></w:p>
</w:body></w:document>`)
	got := OOXMLPlainText(raw)
	want := "SYSTEM BRIEF & DAILY LOG\n> Reminder"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWriteRTFFromDocxPlainTextDecodesEntities(t *testing.T) {
	dir := t.TempDir()
	docx := filepath.Join(dir, "applied.docx")
	raw := writeTestDocxZip(t, `<w:document><w:body>
<w:p><w:r><w:t>SYSTEM BRIEF &amp; DAILY LOG</w:t></w:r></w:p>
<w:p><w:r><w:t>&gt; Reminder</w:t></w:r></w:p>
</w:body></w:document>`)
	if err := os.WriteFile(docx, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "plain.rtf")
	if err := WriteRTFFromDocxPlainText(docx, out); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "SYSTEM BRIEF & DAILY LOG") {
		t.Fatalf("plain rtf missing decoded title: %q", truncateBytes(body, 200))
	}
	if strings.Contains(text, "&amp;") || strings.Contains(text, "&gt;") {
		t.Fatalf("plain rtf leaked XML entities: %q", truncateBytes(body, 200))
	}
}

func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
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
