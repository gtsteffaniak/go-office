package office_test

import (
	"os"
	"path/filepath"
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
	t.Setenv("GO_OFFICE_ASSETS", "/tmp/legacy")
	if got := office.AssetDirFromEnv("fallback"); got != "/tmp/legacy" {
		t.Fatalf("GO_OFFICE_ASSETS got %q", got)
	}
}
