package convert

import "strings"

// Output format codes from OfficeFileFormats.h (AVS_OFFICESTUDIO_FILE_IMAGE_*).
const (
	FormatOutputJPG = 0x0401
	FormatOutputPNG = 0x0405
	FormatOutputPDF = 0x0201
	FormatOutputZIP = 0x0809 // AVS_OFFICESTUDIO_FILE_OTHER_ZIP
)

// FormatOutputExtension maps converter outputtype to ONLYOFFICE format code.
func FormatOutputExtension(ext string) int {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "jpg", "jpeg":
		return FormatOutputJPG
	case "png":
		return FormatOutputPNG
	case "pdf":
		return FormatOutputPDF
	case "zip":
		return FormatOutputZIP
	default:
		return 0
	}
}

// IsImageOutput reports whether outputtype produces raster images.
func IsImageOutput(ext string) bool {
	switch strings.TrimPrefix(strings.ToLower(ext), ".") {
	case "jpg", "jpeg", "png", "bmp", "gif":
		return true
	default:
		return false
	}
}
