package convert

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditorBinReusableRequiresMatchingSourceHash(t *testing.T) {
	dir := t.TempDir()
	if editorBinReusable(dir, "abc") {
		t.Fatal("empty cache should not be reusable")
	}
	if err := os.WriteFile(filepath.Join(dir, "Editor.bin"), []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if editorBinReusable(dir, "abc") {
		t.Fatal("Editor.bin without source hash must not be reused after the CSV changes")
	}
	if err := writeSourceHash(dir, "abc"); err != nil {
		t.Fatal(err)
	}
	if !editorBinReusable(dir, "abc") {
		t.Fatal("matching source hash should reuse Editor.bin")
	}
	if editorBinReusable(dir, "def") {
		t.Fatal("changed source must force reconversion")
	}
}

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.csv")
	if err := os.WriteFile(p, []byte("a,b\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, err := fileSHA256(p)
	if err != nil || h1 == "" {
		t.Fatalf("hash: %q %v", h1, err)
	}
	if err = os.WriteFile(p, []byte("a,b\n1,3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, err := fileSHA256(p)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Fatal("content change should change source hash")
	}
}
