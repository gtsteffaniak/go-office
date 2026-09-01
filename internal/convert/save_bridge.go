package convert

import "strings"

// Save-bridge routing for coauthoring persist (Editor.bin + changes → file on disk).
//
// x2t is always used to OPEN documents (file → Editor.bin), except browser/PDF formats.
// On SAVE with pending changes, x2t's direct Editor.bin→target export often skips
// apply_changes for legacy, flat-text, and ODF targets. Those formats must round-trip
// through the editor canvas native OOXML type first:
//
//   word canvas  → docx  (rtf, doc, dot, odt — not .txt; see below)
//   cell canvas  → xlsx
//   slide canvas → pptx
//
// .txt is plain UTF-8: open and save use the native txt ↔ word-canvas x2t path with
// apply_changes (same model as docx/xlsx direct OOXML saves). No docx round-trip.
//
// On OPEN, legacy/ODF word formats (rtf, doc, dot, odt) convert to docx before
// Editor.bin so coauthoring change blobs match the docx apply_changes path.
//
// Only OOXML container targets are safe for direct fromEditor(..., fromChanges=true):
// docx, dotx, dotm, xlsx, xlsm, pptx, pptm.
//
// Browser formats (pdf, …) reject PersistDocument entirely — no x2t save path.

func spreadsheetSaveNeedsXlsxBridge(ext string) bool {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "csv", "tsv", "scsv", "xls", "ods":
		return true
	default:
		return false
	}
}

func wordSaveNeedsDocxBridge(ext string) bool {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "rtf", "doc", "dot", "odt":
		return true
	default:
		return false
	}
}

func slideSaveNeedsPptxBridge(ext string) bool {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "ppt", "odp":
		return true
	default:
		return false
	}
}

// DirectSaveWithChangesOK reports formats where x2t Editor.bin→same-family OOXML
// export applies coauthoring patches without an intermediate bridge.
func DirectSaveWithChangesOK(ext string) bool {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "docx", "dotx", "dotm", "xlsx", "xlsm", "pptx", "pptm", "txt":
		return true
	default:
		return false
	}
}

// editorImportSourceHash extends a content hash with the open/import pipeline so
// cached Editor.bin is invalidated when bridging changes (e.g. rtf→docx prelude).
func editorImportSourceHash(contentHash, ext string) string {
	if wordSaveNeedsDocxBridge(ext) {
		return contentHash + ":open-docx-v1"
	}
	return contentHash
}

// csvNeedsXlsxBridge is kept for open-path CSV normalization (sheet id on import).
func csvNeedsXlsxBridge(ext string) bool {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "csv", "tsv", "scsv":
		return true
	default:
		return false
	}
}
