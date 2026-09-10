package assetfetch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

var (
	cellCustomXMLHistoryBug = []byte("isMainLogicDocument?this.customXmlManager=new AscWord.CustomXmlManager(this):AscFormat.ExecuteNoHistory(function(){this.customXmlManager=new AscWord.CustomXmlManager(this)},this,[],!0)")
	cellCustomXMLHistoryFix = []byte("AscFormat.ExecuteNoHistory(function(){this.customXmlManager=new AscWord.CustomXmlManager(this)},this,[],!0)")
)

// patchSDKJS fixes release-specific browser bugs in the extracted SDK assets.
func patchSDKJS(outDir string) error {
	path := filepath.Join(outDir, "sdkjs", "cell", "sdk-all.js")
	return patchSDKJSBundle(path)
}

func patchSDKJSBundle(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("assetfetch: read sdkjs patch target: %w", err)
	}
	if bytes.Contains(data, cellCustomXMLHistoryFix) &&
		!bytes.Contains(data, cellCustomXMLHistoryBug) {
		return nil
	}
	if count := bytes.Count(data, cellCustomXMLHistoryBug); count != 1 {
		return fmt.Errorf("assetfetch: unsupported sdkjs bundle %s", path)
	}
	data = bytes.Replace(data, cellCustomXMLHistoryBug, cellCustomXMLHistoryFix, 1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("assetfetch: write sdkjs patch target: %w", err)
	}
	return nil
}
