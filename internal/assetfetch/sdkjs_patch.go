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
	wordMetadataHistoryBug  = []byte("this.CustomProperties=new AscCommon.CCustomProperties,this.customXmlManager=new AscWord.CustomXmlManager(this)")
	wordMetadataHistoryFix  = []byte("AscFormat.ExecuteNoHistory(function(){this.CustomProperties=new AscCommon.CCustomProperties,this.customXmlManager=new AscWord.CustomXmlManager(this)},this,[],!0)")
)

// patchSDKJS fixes release-specific browser bugs in the extracted SDK assets.
func patchSDKJS(outDir string) error {
	if err := patchSDKJSBundle(
		filepath.Join(outDir, "sdkjs", "cell", "sdk-all.js"),
		cellCustomXMLHistoryBug,
		cellCustomXMLHistoryFix,
	); err != nil {
		return err
	}
	return patchSDKJSBundle(
		filepath.Join(outDir, "sdkjs", "word", "sdk-all.js"),
		wordMetadataHistoryBug,
		wordMetadataHistoryFix,
	)
}

func patchSDKJSBundle(path string, buggy, fixed []byte) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("assetfetch: read sdkjs patch target: %w", err)
	}
	if bytes.Contains(data, fixed) && !bytes.Contains(data, buggy) {
		return nil
	}
	if count := bytes.Count(data, buggy); count != 1 {
		return fmt.Errorf("assetfetch: unsupported sdkjs bundle %s", path)
	}
	data = bytes.Replace(data, buggy, fixed, 1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("assetfetch: write sdkjs patch target: %w", err)
	}
	return nil
}
