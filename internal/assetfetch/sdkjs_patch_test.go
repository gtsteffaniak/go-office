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

	// Asset checks run on every build, so the patch must be idempotent.
	if err := patchSDKJS(root); err != nil {
		t.Fatalf("second patch: %v", err)
	}
}

func TestPatchSDKJSDisablesWordMetadataInitializationHistory(t *testing.T) {
	root := t.TempDir()
	wordDir := filepath.Join(root, "sdkjs", "word")
	if err := os.MkdirAll(wordDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wordDir, "sdk-all.js")
	input := append([]byte("before,"), wordMetadataHistoryBug...)
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
	if !bytes.Contains(got, wordMetadataHistoryFix) {
		t.Fatal("Word metadata constructors were not moved outside history")
	}
	if err := patchSDKJS(root); err != nil {
		t.Fatalf("second patch: %v", err)
	}
}

func TestPatchSDKJSRejectsUnknownBundle(t *testing.T) {
	root := t.TempDir()
	cellDir := filepath.Join(root, "sdkjs", "cell")
	if err := os.MkdirAll(cellDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cellDir, "sdk-all.js"), []byte("unknown"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchSDKJS(root); err == nil {
		t.Fatal("expected unsupported bundle error")
	}
}
