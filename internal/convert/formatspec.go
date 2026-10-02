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
	// assemblyRollback mirrors ONLYOFFICE services.CoAuthoring.server.assemblyFormatAsOrigin
	// (default true since v7.0): assemble to the native OOXML bridge format, then *attempt*
	// conversion back to the original format. When that conversion fails, persist the OOXML
	// bridge bytes at the requested path instead of failing the save ("rollback to save
	// changes to ooxml").
	//
	// Formats x2t cannot emit are listed here. Empirically verified against the bundled x2t:
	// xlsx→xls exits 88; hence .xls must roll back. Formats x2t *can* emit (.ods, .rtf, .docx)
	// are not listed: for those a failed conversion is a real error and must surface.
	assemblyRollback bool
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
	// XLS: x2t cannot write binary Excel (verified: xlsx→xls exits 88). Apply changes to
	// changes-applied.xlsx and then roll back to those bytes at the .xls path
	// (assemblyFormatAsOrigin behaviour). The conversion is still attempted first so that a
	// future x2t that can write BIFF is used automatically.
	case "xls":
		return formatSpec{saveBridge: bridgeXLSX, assemblyRollback: true}
	// ODS: x2t *can* write OpenDocument (verified: xlsx→ods succeeds). No rollback — a failed
	// conversion here is a real error and must not be silently replaced with xlsx bytes.
	case "ods":
		return formatSpec{saveBridge: bridgeXLSX}
	// RTF: native import on open; save applies changes to docx then x2t docx→rtf.
	case "rtf":
		return formatSpec{saveBridge: bridgeDOCX}
	// ODT: open via docx prelude; save uses direct reverse (see saveDirectReverse).
	case "odt":
		return formatSpec{saveBridge: bridgeDOCX, openPrelude: bridgeDOCX, saveDirectReverse: true}
	// DOC/DOT: x2t cannot emit binary Word (exit 80). Open via docx prelude; save applies
	// changes to changes-applied.docx then rolls back to OOXML bytes when docx→doc fails.
	case "doc", "dot":
		return formatSpec{saveBridge: bridgeDOCX, openPrelude: bridgeDOCX, assemblyRollback: true}
	// PPT: x2t cannot emit binary PowerPoint (exit 88). PPTX alone is directly writable.
	case "ppt":
		return formatSpec{saveBridge: bridgePPTX, assemblyRollback: true}
	case "odp":
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

// assemblyRollbackEnabled reports whether a failed bridge→original conversion should be
// recovered by persisting the OOXML bridge bytes at the requested path, mirroring
// ONLYOFFICE assemblyFormatAsOrigin ("rollback to save changes to ooxml").
func assemblyRollbackEnabled(ext string) bool {
	return lookupFormat(ext).assemblyRollback
}

// legacyWordBinaryExt reports formats x2t cannot write (binary Word .doc/.dot exit 80).
func legacyWordBinaryExt(ext string) bool {
	switch normExt(ext) {
	case "doc", "dot":
		return true
	default:
		return false
	}
}

// legacySlideBinaryExt reports formats x2t cannot write (binary PowerPoint .ppt exit 88).
func legacySlideBinaryExt(ext string) bool {
	switch normExt(ext) {
	case "ppt":
		return true
	default:
		return false
	}
}

// legacySpreadsheetBinaryExt reports formats x2t cannot write (binary Excel .xls exit 88).
func legacySpreadsheetBinaryExt(ext string) bool {
	switch normExt(ext) {
	case "xls":
		return true
	default:
		return false
	}
}

// bridgeRecoverableExt reports whether the bridge bytes can stand in for ext when the
// bridge→ext conversion failed. This is the single source of truth for the
// assemblyFormatAsOrigin rollback, replacing per-family ad-hoc branches.
func bridgeRecoverableExt(bridge SaveBridge, ext string) bool {
	if !assemblyRollbackEnabled(ext) {
		return false
	}
	switch bridge {
	case bridgeDOCX:
		return legacyWordBinaryExt(ext)
	case bridgePPTX:
		return legacySlideBinaryExt(ext)
	case bridgeXLSX:
		return legacySpreadsheetBinaryExt(ext)
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
