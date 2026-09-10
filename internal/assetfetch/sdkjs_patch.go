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

	wordInitEditorBug = []byte("asc_docs_api.prototype.InitEditor=function(){this.WordControl.m_oLogicDocument=new AscCommonWord.CDocument(this.WordControl.m_oDrawingDocument),this.WordControl.m_oDrawingDocument.m_oLogicDocument=this.WordControl.m_oLogicDocument")
	wordInitEditorFix = []byte("asc_docs_api.prototype.InitEditor=function(){AscFormat.ExecuteNoHistory(function(){this.WordControl.m_oLogicDocument=new AscCommonWord.CDocument(this.WordControl.m_oDrawingDocument)},this,[],!0),this.WordControl.m_oDrawingDocument.m_oLogicDocument=this.WordControl.m_oLogicDocument")

	slideInitEditorBug = []byte("asc_docs_api.prototype.InitEditor=function(){this.WordControl.m_oLogicDocument=new AscCommonSlide.CPresentation(this.WordControl.m_oDrawingDocument),this.WordControl.m_oDrawingDocument.m_oLogicDocument=this.WordControl.m_oLogicDocument")
	slideInitEditorFix = []byte("asc_docs_api.prototype.InitEditor=function(){AscFormat.ExecuteNoHistory(function(){this.WordControl.m_oLogicDocument=new AscCommonSlide.CPresentation(this.WordControl.m_oDrawingDocument)},this,[],!0),this.WordControl.m_oDrawingDocument.m_oLogicDocument=this.WordControl.m_oLogicDocument")
)

// patchSDKJS fixes release-specific browser bugs in the extracted SDK assets.
func patchSDKJS(outDir string) error {
	patches := []struct {
		path     string
		buggy    []byte
		fixed    []byte
		optional bool
	}{
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all.js"), cellCustomXMLHistoryBug, cellCustomXMLHistoryFix, false},
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all-min.js"), cellCustomXMLHistoryBug, cellCustomXMLHistoryFix, true},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordInitEditorBug, wordInitEditorFix, false},
		{filepath.Join(outDir, "sdkjs", "slide", "sdk-all-min.js"), slideInitEditorBug, slideInitEditorFix, false},
	}
	for _, patch := range patches {
		if err := patchSDKJSBundle(patch.path, patch.buggy, patch.fixed, patch.optional); err != nil {
			return err
		}
	}
	return nil
}

func patchSDKJSBundle(path string, buggy, fixed []byte, optional bool) error {
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
	count := bytes.Count(data, buggy)
	if count == 0 {
		if optional {
			return nil
		}
		return fmt.Errorf("assetfetch: unsupported sdkjs bundle %s", path)
	}
	if count != 1 {
		return fmt.Errorf("assetfetch: unsupported sdkjs bundle %s", path)
	}
	data = bytes.Replace(data, buggy, fixed, 1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("assetfetch: write sdkjs patch target: %w", err)
	}
	return nil
}
