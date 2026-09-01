package convert

import (
	"os"
	"path/filepath"
	"strings"
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

func TestWriteRunDoctRendererConfig(t *testing.T) {
	assetDir := t.TempDir()
	runDir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"sdkjs/common/Native/native.js",
		"sdkjs/common/Native/jquery_native.js",
		"web-apps/vendor/xregexp/xregexp-all-min.js",
		"sdkjs",
		"dictionaries",
	} {
		path := filepath.Join(assetDir, filepath.FromSlash(rel))
		if strings.HasSuffix(rel, "sdkjs") || strings.HasSuffix(rel, "dictionaries") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeRunDoctRendererConfig(runDir, assetDir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(runDir, "DoctRenderer.config"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.Contains(body, "<allfonts>./AllFonts.js</allfonts>") {
		t.Fatalf("missing local allfonts: %s", body)
	}
	if !strings.Contains(body, "sdkjs</sdkjs>") {
		t.Fatalf("expected sdkjs path from run dir: %s", body)
	}
}
