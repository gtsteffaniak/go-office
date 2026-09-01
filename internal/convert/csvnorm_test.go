package convert

import "testing"

func TestNormalizePersistedOutputTxtStripsBOM(t *testing.T) {
	raw := append([]byte{0xef, 0xbb, 0xbf}, []byte("hello\r\nworld")...)
	got := NormalizePersistedOutput("txt", raw)
	want := "hello\nworld"
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
