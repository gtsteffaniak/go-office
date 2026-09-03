package convert

// Test hooks for convert_test (linux integration tests).
var ExportPrepareX2TRunDir = (*Converter).prepareX2TRunDir
var ExportConvertOfficeInner = (*Converter).convertOfficeInner
var ExportFromEditorInner = (*Converter).fromEditorInner

func ExportSeedAllFonts(c *Converter) []byte {
	return append([]byte(nil), c.seedAllFonts...)
}

func ExportAllFontFilePaths(src []byte) []string {
	return allFontFilePaths(src)
}
