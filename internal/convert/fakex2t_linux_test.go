//go:build linux

package convert

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFakeX2TValidatesIsolatedFontPaths(t *testing.T) {
	assets := minimalTestAssets(t)
	fake := &FakeX2T{}
	conv, err := New(Options{AssetDir: assets, Limit: 1, Runner: fake.Run})
	if err != nil {
		t.Fatal(err)
	}

	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, "Editor.bin"), []byte("fake-editor"), 0o644); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["change"]`), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(cacheDir, "saved.docx")
	ctx := context.Background()
	if err := conv.SaveChanges(ctx, cacheDir, dest, "docx"); err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	if len(fake.Tasks) == 0 {
		t.Fatal("expected fake x2t invocations")
	}

	var fromChanges *FakeX2TTask
	for i := range fake.Tasks {
		if fake.Tasks[i].FromChanges {
			fromChanges = &fake.Tasks[i]
			break
		}
	}
	if fromChanges == nil {
		t.Fatalf("expected a fromChanges task, got %#v", fake.Tasks)
	}
	if fromChanges.FontDir != fromChanges.IsolatedDir {
		t.Fatalf("fromChanges font dir must be isolated run dir: fontDir=%q isolated=%q", fromChanges.FontDir, fromChanges.IsolatedDir)
	}
	if fromChanges.AllFontsPath != filepath.Join(fromChanges.IsolatedDir, "AllFonts.js") {
		t.Fatalf("fromChanges all fonts must be under run dir: %q", fromChanges.AllFontsPath)
	}
}

func minimalTestAssets(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	binDir := filepath.Join(root, "converter", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	x2t := filepath.Join(binDir, "x2t")
	if err := os.WriteFile(x2t, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "AllFonts.js"), []byte("window.__all_fonts=[];"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "font_selection.bin"), []byte{0}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sdkjs", "slide", "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "core-fonts"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}
