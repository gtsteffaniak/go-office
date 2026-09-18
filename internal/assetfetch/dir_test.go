package assetfetch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearDirContentsLeavesDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"web-apps", "sdkjs", ".extracted"} {
		path := filepath.Join(dir, name)
		if name == ".extracted" {
			if err := os.WriteFile(path, []byte("v"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := clearDirContents(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty dir, got %d entries", len(entries))
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsDir() {
		t.Fatal("expected dir to still exist")
	}
}

func TestClearDirContentsMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	if err := clearDirContents(dir); err != nil {
		t.Fatal(err)
	}
}
