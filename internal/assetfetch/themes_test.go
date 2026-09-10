package assetfetch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSlideThemesJS(t *testing.T) {
	root := t.TempDir()
	if err := ensureSlideThemesJS(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sdkjs", "slide", "themes", "themes.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty themes.js stub")
	}
	if err := ensureSlideThemesJS(root); err != nil {
		t.Fatal(err)
	}
}
