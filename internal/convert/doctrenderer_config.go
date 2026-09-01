package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fixDoctRendererConfig rewrites DocumentServer-relative paths for go-office's
// flattened assets layout (converter/bin is two levels below assets/, not three).
func fixDoctRendererConfig(binDir string) error {
	path := filepath.Join(binDir, "DoctRenderer.config")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	updated := strings.ReplaceAll(string(data), "../../../", "../../")
	if updated == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

// writeRunDoctRendererConfig writes a per-run DoctRenderer.config so x2t loads
// ./AllFonts.js from an isolated directory instead of the shared converter/bin copy.
func writeRunDoctRendererConfig(runDir, assetDir string) error {
	rel := func(abs string) string {
		p, err := filepath.Rel(runDir, abs)
		if err != nil {
			return abs
		}
		return filepath.ToSlash(p)
	}
	nativeJS := filepath.Join(assetDir, "sdkjs", "common", "Native", "native.js")
	jqueryNative := filepath.Join(assetDir, "sdkjs", "common", "Native", "jquery_native.js")
	xregexp := filepath.Join(assetDir, "web-apps", "vendor", "xregexp", "xregexp-all-min.js")
	sdkjs := filepath.Join(assetDir, "sdkjs")
	dictionaries := filepath.Join(assetDir, "dictionaries")
	xml := fmt.Sprintf(`<Settings>
<file>%s</file>
<file>%s</file>
<allfonts>./AllFonts.js</allfonts>
<file>%s</file>
<sdkjs>%s</sdkjs>
<dictionaries>%s</dictionaries>
</Settings>
`, rel(nativeJS), rel(jqueryNative), rel(xregexp), rel(sdkjs), rel(dictionaries))
	return os.WriteFile(filepath.Join(runDir, "DoctRenderer.config"), []byte(xml), 0o644)
}
