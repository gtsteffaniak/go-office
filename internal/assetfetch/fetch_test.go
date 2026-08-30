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

