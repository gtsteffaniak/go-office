package convert

import "strings"

// SaveBridge is the OOXML canvas type used for apply_changes before converting to the target format.
type SaveBridge string

const (
	bridgeNone SaveBridge = ""
	bridgeXLSX SaveBridge = "xlsx"
	bridgeDOCX SaveBridge = "docx"
	bridgePPTX SaveBridge = "pptx"
)

type formatSpec struct {
	saveBridge        SaveBridge
	openPrelude       SaveBridge
	openNormCSV       bool
	directSave        bool
	// saveDirectReverse: apply_changes writes the target format directly via reverse x2t,
	// skipping office-to-office step 2. Used for RTF/ODT where x2t supports Editor.bin→target
	// but docx→target would be redundant or less reliable under concurrent load.
	saveDirectReverse bool
}

func normExt(ext string) string {
	return strings.TrimPrefix(strings.ToLower(ext), ".")
}

func lookupFormat(ext string) formatSpec {
	switch normExt(ext) {
	case "docx", "dotx", "dotm", "xlsx", "xlsm", "pptx", "pptm", "txt":
		return formatSpec{directSave: true}
	case "csv", "tsv", "scsv":
		return formatSpec{saveBridge: bridgeXLSX, openNormCSV: true}
	case "xls", "ods":
		return formatSpec{saveBridge: bridgeXLSX}
	// RTF: native import on open; save applies changes to docx then plain-text RTF export.
	case "rtf":
		return formatSpec{saveBridge: bridgeDOCX}
	// ODT: open via docx prelude; save uses direct reverse (see saveDirectReverse).
	case "odt":
		return formatSpec{saveBridge: bridgeDOCX, openPrelude: bridgeDOCX, saveDirectReverse: true}
	// DOC/DOT: x2t cannot emit binary Word (exit 80). Open via docx prelude; save applies
	// changes to changes-applied.docx then falls back to OOXML bytes when docx→doc fails.
	case "doc", "dot":
		return formatSpec{saveBridge: bridgeDOCX, openPrelude: bridgeDOCX}
	case "ppt", "odp":
		return formatSpec{saveBridge: bridgePPTX}
	default:
		return formatSpec{}
	}
}

func spreadsheetSaveNeedsXlsxBridge(ext string) bool {
	return lookupFormat(ext).saveBridge == bridgeXLSX
}

func wordSaveNeedsDocxBridge(ext string) bool {
	return lookupFormat(ext).saveBridge == bridgeDOCX
}

func slideSaveNeedsPptxBridge(ext string) bool {
	return lookupFormat(ext).saveBridge == bridgePPTX
}

func DirectSaveWithChangesOK(ext string) bool {
	return lookupFormat(ext).directSave
}

func csvNeedsXlsxBridge(ext string) bool {
	return lookupFormat(ext).openNormCSV
}

func openNeedsDocxPrelude(ext string) bool {
	return lookupFormat(ext).openPrelude == bridgeDOCX
}

func saveBridgeExt(ext string) SaveBridge {
	return lookupFormat(ext).saveBridge
}

func legacyWordSaveDirectReverse(ext string) bool {
	return lookupFormat(ext).saveDirectReverse
}

// legacyWordBinaryExt reports formats x2t cannot write (binary Word .doc/.dot exit 80).
// Save path copies changes-applied.docx as an OOXML fallback — same rollback Document Server
// uses when assemblyFormatAsOrigin cannot convert back to the original legacy format.
func legacyWordBinaryExt(ext string) bool {
	switch normExt(ext) {
	case "doc", "dot":
		return true
	default:
		return false
	}
}

// legacySlideBinaryExt reports formats x2t cannot write (binary PowerPoint .ppt exit 88).
// Save path copies changes-applied.pptx bytes at the legacy path when pptx→ppt fails.
func legacySlideBinaryExt(ext string) bool {
	switch normExt(ext) {
	case "ppt":
		return true
	default:
		return false
	}
}

func editorImportSourceHash(contentHash, ext string) string {
	if openNeedsDocxPrelude(ext) {
		return contentHash + ":open-docx-v1"
	}
	return contentHash
}
