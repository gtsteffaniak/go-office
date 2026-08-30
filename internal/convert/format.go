package convert

import "strings"

// AVS_OFFICESTUDIO_FILE_CANVAS_* output formats for Editor.bin (see OfficeFileFormats.h).
const (
	FormatCanvasWord         = 0x2001
	FormatCanvasSpreadsheet  = 0x2002
	FormatCanvasPresentation = 0x2003
	FormatCanvasPDF          = 0x2004
)

// FormatFromExtension maps a file extension to ONLYOFFICE input format codes.
func FormatFromExtension(ext string) int {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	switch ext {
	case "docx":
		return 0x0041
	case "doc", "dot":
		return 0x0042
	case "odt":
		return 0x0043
	case "rtf":
		return 0x0044
	case "txt":
		return 0x0045
	case "dotx":
		return 0x004c
	case "dotm":
		return 0x004d
	case "pptx":
		return 0x0081
	case "ppt":
		return 0x0082
	case "odp":
		return 0x0083
	case "pptm":
		return 0x0085
	case "xlsx":
		return 0x0101
	case "xls":
		return 0x0102
	case "ods":
		return 0x0103
	case "csv":
		return 0x0104
	case "xlsm":
		return 0x0105
	case "pdf":
		return 0x0201
	default:
		return 0
	}
}

// FormatCanvasTo returns the Editor.bin canvas format for the given source extension.
func FormatCanvasTo(ext string) int {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	switch ext {
	case "xls", "xlsx", "xlsm", "xlsb", "ods", "csv", "tsv":
		return FormatCanvasSpreadsheet
	case "ppt", "pptx", "pptm", "odp":
		return FormatCanvasPresentation
	case "pdf":
		return FormatCanvasPDF
	default:
		return FormatCanvasWord
	}
}
