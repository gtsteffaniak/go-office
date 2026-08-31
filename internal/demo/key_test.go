package demo

import (
	"testing"
	"time"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestDocumentKeyTracksContentNotOnlyMtime(t *testing.T) {
	info := office.FileInfo{Size: 100, ModTime: time.Unix(1_700_000_000, 0)}
	a := documentKey("sample-files/sample.csv", info, "hash-a")
	b := documentKey("sample-files/sample.csv", info, "hash-b")
	if a == b {
		t.Fatal("same mtime with different file bytes must not reuse the editor cache key")
	}
	c := documentKey("sample-files/sample.csv", office.FileInfo{Size: 101, ModTime: info.ModTime}, "hash-a")
	if a == c {
		t.Fatal("size change must change the document key")
	}
}
