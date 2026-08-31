package convert

import "testing"

func TestNormalizeCSVBytes(t *testing.T) {
	raw := []byte{0xef, 0xbb, 0xbf, 'A', ',', 'B', '\r', '\n', '1', ',', '2', '\r', '\n'}
	got := normalizeCSVBytes(raw)
	want := []byte("A,B\n1,2\n")
	if string(got) != string(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}
