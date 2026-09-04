package convert

import "testing"

func TestSaveBridgeRouting(t *testing.T) {
	type row struct {
		ext           string
		xlsx          bool
		docx          bool
		pptx          bool
		direct        bool
		directReverse bool
	}
	cases := []row{
		// OOXML — direct apply_changes OK
		{"docx", false, false, false, true, false},
		{"dotx", false, false, false, true, false},
		{"dotm", false, false, false, true, false},
		{"xlsx", false, false, false, true, false},
		{"xlsm", false, false, false, true, false},
		{"pptx", false, false, false, true, false},
		{"pptm", false, false, false, true, false},
		{"txt", false, false, false, true, false},
		// Flat / legacy / ODF — bridge or direct-reverse save
		{"csv", true, false, false, false, false},
		{"tsv", true, false, false, false, false},
		{"scsv", true, false, false, false, false},
		{"xls", true, false, false, false, false},
		{"ods", true, false, false, false, false},
		// RTF: save via docx bridge; ODT uses direct reverse apply_changes.
		{"rtf", false, true, false, false, false},
		// DOC/DOT: x2t cannot write binary Word (exit 80); save uses docx bridge + OOXML fallback.
		{"doc", false, true, false, false, false},
		{"dot", false, true, false, false, false},
		{"odt", false, true, false, false, true},
		{"ppt", false, false, true, false, false},
		{"odp", false, false, true, false, false},
	}
	for _, tc := range cases {
		if got := spreadsheetSaveNeedsXlsxBridge(tc.ext); got != tc.xlsx {
			t.Fatalf("%s xlsxBridge = %v want %v", tc.ext, got, tc.xlsx)
		}
		if got := wordSaveNeedsDocxBridge(tc.ext); got != tc.docx {
			t.Fatalf("%s docxBridge = %v want %v", tc.ext, got, tc.docx)
		}
		if got := slideSaveNeedsPptxBridge(tc.ext); got != tc.pptx {
			t.Fatalf("%s pptxBridge = %v want %v", tc.ext, got, tc.pptx)
		}
		if got := DirectSaveWithChangesOK(tc.ext); got != tc.direct {
			t.Fatalf("%s direct = %v want %v", tc.ext, got, tc.direct)
		}
		if got := legacyWordSaveDirectReverse(tc.ext); got != tc.directReverse {
			t.Fatalf("%s directReverse = %v want %v", tc.ext, got, tc.directReverse)
		}
	}
}

func TestLegacyBinaryExtFallbacks(t *testing.T) {
	cases := []struct {
		ext         string
		wordLegacy  bool
		slideLegacy bool
		sheetLegacy bool
	}{
		{"doc", true, false, false},
		{"dot", true, false, false},
		{"ppt", false, true, false},
		{"xls", false, false, true},
		{"xlsx", false, false, false},
		{"docx", false, false, false},
	}
	for _, tc := range cases {
		if got := legacyWordBinaryExt(tc.ext); got != tc.wordLegacy {
			t.Fatalf("%s wordLegacy = %v want %v", tc.ext, got, tc.wordLegacy)
		}
		if got := legacySlideBinaryExt(tc.ext); got != tc.slideLegacy {
			t.Fatalf("%s slideLegacy = %v want %v", tc.ext, got, tc.slideLegacy)
		}
		if got := legacySpreadsheetBinaryExt(tc.ext); got != tc.sheetLegacy {
			t.Fatalf("%s sheetLegacy = %v want %v", tc.ext, got, tc.sheetLegacy)
		}
	}
}

func TestEditorImportSourceHash(t *testing.T) {
	base := "abc123"
	if got := editorImportSourceHash(base, "docx"); got != base {
		t.Fatalf("docx hash = %q want %q", got, base)
	}
	if got := editorImportSourceHash(base, "txt"); got != base {
		t.Fatalf("txt hash = %q want %q", got, base)
	}
	if got := editorImportSourceHash(base, "rtf"); got != base {
		t.Fatalf("rtf hash = %q want %q (native import, no docx prelude)", got, base)
	}
	if got := editorImportSourceHash(base, "odt"); got == base {
		t.Fatal("odt hash should include open-docx pipeline tag")
	}
}
