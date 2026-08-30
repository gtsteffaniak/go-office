package assetfetch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/assetfetch"
)

func TestLoadVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "euro-office.version")
	if err := os.WriteFile(path, []byte("EURO_OFFICE_RELEASE=v9.3.4-hotfix.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := assetfetch.LoadVersion(path)
	if err != nil {
		t.Fatal(err)
	}
	if v.Release != "9.3.4-hotfix.1" || v.Protocol != "9.3.4-hotfix.1" {
		t.Fatalf("unexpected version: %+v", v)
	}
}

func TestDebURL(t *testing.T) {
	url := assetfetch.DebURL(assetfetch.Version{Release: "9.3.4-hotfix.1"})
	if url == "" || !strings.Contains(url, "9.3.4-hotfix.1_amd64.deb") {
		t.Fatalf("url = %q", url)
	}
}

func TestIsUpToDateRequiresFonts(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, ".extracted")
	if err := os.WriteFile(marker, []byte("9.3.4-hotfix.1"), 0o644); err != nil {
		t.Fatal(err)
	}
	apiDir := filepath.Join(dir, "web-apps", "apps", "api", "documents")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "api.js.tpl"), []byte("tpl"), 0o644); err != nil {
		t.Fatal(err)
	}
	if assetfetch.IsUpToDate(marker, "9.3.4-hotfix.1", dir) {
		t.Fatal("expected incomplete assets (no fonts) to be out of date")
	}
}

func TestFontGenerationPossibleRequiresImages(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{
		"converter/bin",
		"core-fonts",
		"tools",
	} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "tools", "allfontsgen"), []byte{0}, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "converter", "bin", "x2t"), []byte{0}, 0o755); err != nil {
		t.Fatal(err)
	}
	if assetfetch.FontGenerationPossible(dir) {
		t.Fatal("expected false without sdkjs/common/Images")
	}
}

func TestFindDocumentServerRoot(t *testing.T) {
	root := t.TempDir()
	ds := filepath.Join(root, "var", "www", "euro-office", "documentserver")
	for _, sub := range []string{"web-apps", "sdkjs"} {
		if err := os.MkdirAll(filepath.Join(ds, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := assetfetch.FindDocumentServerRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != ds {
		t.Fatalf("got %q want %q", got, ds)
	}
}

