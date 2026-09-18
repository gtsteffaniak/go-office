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
	// cell/sdk-all.js now also requires the CustomXmlManager constructor patch, so the
	// fixture must carry that anchor too or patchSDKJS correctly rejects the bundle.
	input := append([]byte("before,"), cellCustomXMLHistoryBug...)
	input = append(input, ',')
	input = append(input, cellCustomXMLCtorBug...)
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

// TestPatchSDKJSForwardsRealCoauthoringToken guards the fix for the hardcoded
// "fghhfgsjdgfjs" placeholder. Without it the coauthoring auth packet carries a non-JWT
// string and any document server verifying JWTs rejects every session with
// "token contains an invalid number of segments".
func TestPatchSDKJSForwardsRealCoauthoringToken(t *testing.T) {
	for _, editor := range []string{"cell", "word", "slide"} {
		t.Run(editor, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "sdkjs", editor)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "sdk-all-min.js")
			// The other required anchors for this editor must be present too, or
			// patchSDKJS correctly rejects the bundle as unrecognised.
			var b bytes.Buffer
			b.WriteString("prefix,")
			for _, anchor := range requiredAnchorsFor(editor) {
				b.Write(anchor)
				b.WriteByte(',')
			}
			b.Write(coauthoringTokenBug)
			b.WriteString(",suffix")
			if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := patchSDKJS(root); err != nil {
				t.Fatalf("patchSDKJS: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(got, coauthoringTokenFix) {
				t.Fatalf("%s: token patch not applied; _token does not prefer the config token", editor)
			}
			if !bytes.Contains(got, []byte("this._token=this.jwtOpen||this._token")) {
				t.Fatalf("%s: unexpected patch result: %s", editor, got)
			}

			// Idempotent: a second pass must not double-apply or error.
			if secondErr := patchSDKJS(root); secondErr != nil {
				t.Fatalf("second patchSDKJS: %v", secondErr)
			}
			again, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, again) {
				t.Fatal("patch is not idempotent")
			}
		})
	}
}

// TestPatchSDKJSRejectsMissingTokenAnchor ensures an unrecognised sdkjs bundle fails loudly
// rather than silently shipping a server that cannot verify JWTs.
func TestPatchSDKJSRejectsMissingTokenAnchor(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sdkjs", "word")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// All other required patches present, but the token anchor removed.
	var b bytes.Buffer
	for _, p := range []struct{ bug []byte }{
		{wordInitEditorBug}, {wordBeforeOpenBug}, {wordOpenFromBinBug}, {wordOpenFromZipBug},
	} {
		b.Write(p.bug)
		b.WriteByte(',')
	}
	if err := os.WriteFile(filepath.Join(dir, "sdk-all-min.js"), b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchSDKJS(root); err == nil {
		t.Fatal("expected an error when the coauthoring token anchor is missing")
	}
}

// requiredAnchorsFor returns the non-optional patch anchors patchSDKJS expects for an editor.
func requiredAnchorsFor(editor string) [][]byte {
	switch editor {
	case "cell":
		return [][]byte{cellOpenFromBinNoInitBug}
	case "word":
		return [][]byte{wordInitEditorBug, wordBeforeOpenBug, wordOpenFromBinBug, wordOpenFromZipBug}
	case "slide":
		return [][]byte{slideInitEditorBug, slideOpenFromBinBug}
	default:
		return nil
	}
}

// TestPatchSDKJSKeepsCustomXmlManagerOutOfHistory is the regression test for the
// spreadsheet load crash:
//
//	TypeError: this.NewClass.Write_ToBinary2 is not a function
//	  at CChangesTableIdAdd.WriteToBinary (sdk-all-min.js)
//	  ... at CHistory.Add ... at new CustomXmlManager
//
// CustomXmlManager registered itself in the global object table while undo history was
// enabled, so the history tried to serialise a half-constructed object. The earlier fix
// wrapped the *caller*, which was not the failing frame; the constructor must be patched.
func TestPatchSDKJSKeepsCustomXmlManagerOutOfHistory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sdkjs", "cell")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sdk-all.js")
	input := append([]byte("prefix,"), cellCustomXMLHistoryBug...)
	input = append(input, ',')
	input = append(input, cellCustomXMLCtorBug...)
	input = append(input, []byte(",suffix")...)
	if err := os.WriteFile(path, input, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchSDKJS(root); err != nil {
		t.Fatalf("patchSDKJS: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, cellCustomXMLCtorFix) {
		t.Fatalf("CustomXmlManager constructor still registers inside undo history: %s", got)
	}
	if bytes.Contains(got, cellCustomXMLCtorBug) {
		t.Fatal("buggy constructor text still present")
	}
	// The registration must survive — only its history recording is suppressed.
	if !bytes.Contains(got, []byte("AscCommon.g_oTableId.Add(this,this.Id)")) {
		t.Fatal("table registration was removed entirely; only history suppression is wanted")
	}

	if secondErr := patchSDKJS(root); secondErr != nil {
		t.Fatalf("second patchSDKJS: %v", secondErr)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, again) {
		t.Fatal("patch is not idempotent")
	}
}
