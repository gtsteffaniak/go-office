package office

import "testing"

func TestHeadPreview(t *testing.T) {
	raw := []byte("Index,Customer Id\r\n3,AAAA\n4,BBBB\n")
	got := headPreview(raw)
	if got != "Index,Customer Id\n3,AAAA" {
		t.Fatalf("headPreview = %q", got)
	}
	if cell := firstCSVDataCell(raw); cell != "3" {
		t.Fatalf("firstCSVDataCell = %q", cell)
	}
}

func TestPersistTextPreviewCSV(t *testing.T) {
	raw := []byte("Index,Customer Id\r\n3,AAAA\n")
	got := persistTextPreview("csv", raw)
	if got != "cell=3 Index,Customer Id\n3,AAAA" {
		t.Fatalf("persistTextPreview csv = %q", got)
	}
}
