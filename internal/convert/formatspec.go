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
	saveBridge  SaveBridge
	openPrelude SaveBridge
	openNormCSV bool
	directSave  bool
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
	case "rtf", "doc", "dot", "odt":
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

func editorImportSourceHash(contentHash, ext string) string {
	if openNeedsDocxPrelude(ext) {
		return contentHash + ":open-docx-v1"
	}
	return contentHash
}
