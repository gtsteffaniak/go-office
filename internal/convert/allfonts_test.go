package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteAllFontsRelocatedAssetDir(t *testing.T) {
	assetDir := t.TempDir()
	fontRel := filepath.Join("core-fonts", "dejavu", "DejaVuSans.ttf")
	if err := os.MkdirAll(filepath.Join(assetDir, "core-fonts", "dejavu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, fontRel), []byte("ttf"), 0o644); err != nil {
		t.Fatal(err)
	}

	src := []byte(`window["__fonts_files"] = [
"/host/old/assets/core-fonts/dejavu/DejaVuSans.ttf",
"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
];
`)
	out := rewriteAllFontsPaths(src, assetDir)
	want := filepath.ToSlash(filepath.Join(assetDir, fontRel))
	if !strings.Contains(string(out), want) {
		t.Fatalf("expected remapped path in AllFonts.js: %s", out)
	}
	if strings.Contains(string(out), "/host/old/assets") {
		t.Fatalf("stale host prefix remains: %s", out)
	}
	if strings.Contains(string(out), "/usr/share/fonts") {
		t.Fatalf("system font path was not remapped to core-fonts: %s", out)
	}
	for _, p := range allFontFilePaths(out) {
		if p != want {
			t.Fatalf("rewritten path %q want %q", p, want)
		}
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("rewritten path missing: %v", err)
		}
	}
}

func TestRewriteAllFontsPathsExported(t *testing.T) {
	assetDir := t.TempDir()
	fontRel := filepath.Join("core-fonts", "dejavu", "DejaVuSans.ttf")
	if err := os.MkdirAll(filepath.Join(assetDir, "core-fonts", "dejavu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, fontRel), []byte("ttf"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := []byte(fmt.Sprintf(`window["__fonts_files"] = [%q];`, "/host/old/assets/core-fonts/dejavu/DejaVuSans.ttf"))
	out := RewriteAllFontsPaths(src, assetDir)
	want := filepath.ToSlash(filepath.Join(assetDir, fontRel))
	if !strings.Contains(string(out), want) {
		t.Fatalf("expected remapped path in AllFonts.js: %s", out)
	}
}
