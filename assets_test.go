package office_test

import (
	"os"
	"path/filepath"
	"testing"

	office "github.com/quantumx-apps/go-office"
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
	t.Setenv("GO_OFFICE_ASSETS", "/tmp/assets")
	if got := office.AssetDirFromEnv("fallback"); got != "/tmp/assets" {
		t.Fatalf("got %q", got)
	}
}
