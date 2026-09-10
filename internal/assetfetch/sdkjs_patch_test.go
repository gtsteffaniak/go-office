package assetfetch

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPatchSDKJSDisablesCustomXMLManagerHistory(t *testing.T) {
	root := t.TempDir()
	cellDir := filepath.Join(root, "sdkjs", "cell")
	if err := os.MkdirAll(cellDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cellDir, "sdk-all.js")
	input := append([]byte("before,"), cellCustomXMLHistoryBug...)
	input = append(input, []byte(",after")...)
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchSDKJS(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, cellCustomXMLHistoryFix) {
		t.Fatal("CustomXmlManager constructor was not moved outside history")
	}

	if err := patchSDKJS(root); err != nil {
		t.Fatalf("second patch: %v", err)
	}
}

func TestPatchSDKJSWrapsWordInitEditorInNoHistory(t *testing.T) {
	root := t.TempDir()
	wordDir := filepath.Join(root, "sdkjs", "word")
	if err := os.MkdirAll(wordDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wordDir, "sdk-all-min.js")
	input := append([]byte("before,"), wordInitEditorBug...)
	input = append(input, []byte(",after")...)
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchSDKJSBundle(path, wordInitEditorBug, wordInitEditorFix, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, wordInitEditorFix) {
		t.Fatal("Word InitEditor was not wrapped in ExecuteNoHistory")
	}
}

func TestPatchSDKJSWrapsSlideInitEditorInNoHistory(t *testing.T) {
	root := t.TempDir()
	slideDir := filepath.Join(root, "sdkjs", "slide")
	if err := os.MkdirAll(slideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(slideDir, "sdk-all-min.js")
	input := append([]byte("before,"), slideInitEditorBug...)
	input = append(input, []byte(",after")...)
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchSDKJSBundle(path, slideInitEditorBug, slideInitEditorFix, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, slideInitEditorFix) {
		t.Fatal("Slide InitEditor was not wrapped in ExecuteNoHistory")
	}
}

func TestPatchSDKJSWrapsWordOpenDocumentFromBinInNoHistory(t *testing.T) {
	root := t.TempDir()
	wordDir := filepath.Join(root, "sdkjs", "word")
	if err := os.MkdirAll(wordDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wordDir, "sdk-all-min.js")
	input := append([]byte("before,"), wordOpenFromBinBug...)
	input = append(input, []byte(",after")...)
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchSDKJSBundle(path, wordOpenFromBinBug, wordOpenFromBinFix, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, wordOpenFromBinFix) {
		t.Fatal("Word OpenDocumentFromBin was not wrapped in ExecuteNoHistory")
	}
}

func TestPatchSDKJSRejectsUnknownBundle(t *testing.T) {
	root := t.TempDir()
	wordDir := filepath.Join(root, "sdkjs", "word")
	if err := os.MkdirAll(wordDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wordDir, "sdk-all-min.js"), []byte("unknown"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchSDKJS(root); err == nil {
		t.Fatal("expected unsupported bundle error")
	}
}
