package convert

// Test hooks for convert_test (linux integration tests).
var ExportPrepareX2TRunDir = (*Converter).prepareX2TRunDir

func ExportSeedAllFonts(c *Converter) []byte {
	return append([]byte(nil), c.seedAllFonts...)
}

func ExportAllFontFilePaths(src []byte) []string {
	return allFontFilePaths(src)
}
