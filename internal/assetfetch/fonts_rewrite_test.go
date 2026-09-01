package assetfetch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteAllFontsInTreeRemapsPaths(t *testing.T) {
	dir := t.TempDir()
	fontRel := filepath.Join("core-fonts", "dejavu", "DejaVuSans.ttf")
	if err := os.MkdirAll(filepath.Join(dir, "core-fonts", "dejavu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fontRel), []byte("ttf"), 0o644); err != nil {
		t.Fatal(err)
	}

	stale := []byte(`window["__fonts_files"] = ["/host/old/assets/core-fonts/dejavu/DejaVuSans.ttf"];`)
	binDir := filepath.Join(dir, "converter", "bin")
	sdkDir := filepath.Join(dir, "sdkjs", "common")
	for _, p := range []string{binDir, sdkDir} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(binDir, "AllFonts.js"), stale, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdkDir, "AllFonts.js"), stale, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := rewriteAllFontsInTree(dir); err != nil {
		t.Fatal(err)
	}

	want := filepath.ToSlash(filepath.Join(dir, fontRel))
	for _, rel := range []string{
		filepath.Join("converter", "bin", "AllFonts.js"),
		filepath.Join("sdkjs", "common", "AllFonts.js"),
	} {
		body, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), want) {
			t.Fatalf("%s not rewritten: %s", rel, body)
		}
		if strings.Contains(string(body), "/host/old/assets") {
			t.Fatalf("%s still has stale prefix: %s", rel, body)
		}
	}

	if err := rewriteAllFontsInTree(dir); err != nil {
		t.Fatal(err)
	}
}
