package assetfetch

import (
	"os"
	"path/filepath"
)

const slideThemesJSStub = "/* go-office: stub slide themes bundle (vendor tree has themes/src only) */\n"

// ensureSlideThemesJS creates sdkjs/slide/themes/themes.js when the vendor package omits it.
// The presentation editor loads this script at startup; a missing file breaks slide open.
func ensureSlideThemesJS(outDir string) error {
	path := filepath.Join(outDir, "sdkjs", "slide", "themes", "themes.js")
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(slideThemesJSStub), 0o644)
}
