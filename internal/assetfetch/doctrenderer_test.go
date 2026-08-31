package assetfetch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixDoctRendererConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DoctRenderer.config")
	if err := os.WriteFile(path, []byte("<sdkjs>../../../sdkjs</sdkjs>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fixDoctRendererConfig(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "<sdkjs>../../sdkjs</sdkjs>\n"
	if string(got) != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
