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
		// RTF/ODT: x2t can apply_changes straight to the target; docx→rtf/odt office step is redundant.
		{"rtf", false, true, false, false, true},
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

func TestEditorImportSourceHash(t *testing.T) {
	base := "abc123"
	if got := editorImportSourceHash(base, "docx"); got != base {
		t.Fatalf("docx hash = %q want %q", got, base)
	}
	if got := editorImportSourceHash(base, "txt"); got != base {
		t.Fatalf("txt hash = %q want %q", got, base)
	}
	if got := editorImportSourceHash(base, "rtf"); got == base {
		t.Fatal("rtf hash should include open-docx pipeline tag")
	}
}
