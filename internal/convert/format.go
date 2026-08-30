package convert

// AVS_OFFICESTUDIO_FILE_CANVAS is the x2t output format for Editor.bin.
const FormatCanvas = 0x2000

// FormatFromExtension maps a file extension to ONLYOFFICE input format codes.
func FormatFromExtension(ext string) int {
	switch ext {
	case "docx":
		return 0x0041
	case "doc":
		return 0x0042
	case "odt":
		return 0x0043
	case "rtf":
		return 0x0044
	case "txt":
		return 0x0045
	case "xlsx":
		return 0x0101
	case "xls":
		return 0x0102
	case "pptx":
		return 0x0081
	case "ppt":
		return 0x0082
	default:
		return 0
	}
}
