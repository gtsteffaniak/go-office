package office_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestReadAssetVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("9.3.4-hotfix.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := office.ReadAssetVersion(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "9.3.4-hotfix.1" {
		t.Fatalf("got %q", got)
	}
}

func TestAssetDirFromEnv(t *testing.T) {
	t.Setenv("OFFICE_ASSETS", "/tmp/office-assets")
	if got := office.AssetDirFromEnv("fallback"); got != "/tmp/office-assets" {
		t.Fatalf("OFFICE_ASSETS got %q", got)
	}
	t.Setenv("OFFICE_ASSETS", "")
	if got := office.AssetDirFromEnv("fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
	t.Setenv("GO_OFFICE_ASSETS", "/tmp/legacy")
	if got := office.AssetDirFromEnv("fallback"); got != "fallback" {
		t.Fatalf("GO_OFFICE_ASSETS must not be used, got %q", got)
	}
}

func TestDiscoverAssetsFindsPreferredDir(t *testing.T) {
	dir := t.TempDir()
	writeMinimalValidAssets(t, dir)

	bundle, ok, err := office.DiscoverAssets(office.AssetOptions{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected assets to be discovered")
	}
	if bundle.Dir != dir && bundle.Dir != filepath.Clean(dir) {
		abs, _ := filepath.Abs(dir)
		if bundle.Dir != abs {
			t.Fatalf("dir = %q want %q", bundle.Dir, dir)
		}
	}
	if bundle.Version != "9.3.4-hotfix.1" {
		t.Fatalf("version = %q", bundle.Version)
	}
}

func TestDiscoverAssetsMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OFFICE_ASSETS", "/should/not/be/used")
	_, ok, err := office.DiscoverAssets(office.AssetOptions{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no assets")
	}
}

func TestDiscoverAssetsFromEnvWhenDirEmpty(t *testing.T) {
	dir := t.TempDir()
	writeMinimalValidAssets(t, dir)
	t.Setenv("OFFICE_ASSETS", dir)

	bundle, ok, err := office.DiscoverAssets(office.AssetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected assets from OFFICE_ASSETS")
	}
	if bundle.Dir != dir {
		abs, _ := filepath.Abs(dir)
		if bundle.Dir != abs {
			t.Fatalf("dir = %q want %q", bundle.Dir, dir)
		}
	}
}

func TestValidAssetDirWrapper(t *testing.T) {
	dir := t.TempDir()
	if office.ValidAssetDir(dir) {
		t.Fatal("empty dir should be invalid")
	}
	writeMinimalValidAssets(t, dir)
	if !office.ValidAssetDir(dir) {
		t.Fatal("expected valid assets")
	}
}

func writeMinimalValidAssets(t *testing.T, dir string) {
	t.Helper()
	apiDir := filepath.Join(dir, "web-apps", "apps", "api", "documents")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "api.js"), []byte("api"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(dir, "sdkjs", "common", "AllFonts.js"),
		filepath.Join(dir, "converter", "bin", "font_selection.bin"),
		filepath.Join(dir, "converter", "bin", "AllFonts.js"),
		filepath.Join(dir, "converter", "bin", "x2t"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		size := 1
		if strings.Contains(p, "AllFonts.js") {
			size = 64 * 1024
		}
		if strings.Contains(p, "font_selection.bin") {
			size = 1024
		}
		payload := make([]byte, size)
		payload[0] = 'x'
		if err := os.WriteFile(p, payload, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("9.3.4-hotfix.1"), 0o644); err != nil {
		t.Fatal(err)
	}
}
