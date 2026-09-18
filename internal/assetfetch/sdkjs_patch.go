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

	cellOpenFromBinNoInitBug = []byte("OpenDocumentFromBinNoInit=function(gObject){AscFonts.IsCheckSymbols=!0,(new AscCommonExcel.BinaryFileReader).Read(gObject,this.wbModel),AscFonts.IsCheckSymbols=!1}")
	cellOpenFromBinNoInitFix = []byte("OpenDocumentFromBinNoInit=function(gObject){AscFonts.IsCheckSymbols=!0,AscFormat.ExecuteNoHistory(function(){(new AscCommonExcel.BinaryFileReader).Read(gObject,this.wbModel)},this,[],!0),AscFonts.IsCheckSymbols=!1}")

	wordInitEditorBug = []byte("asc_docs_api.prototype.InitEditor=function(){this.WordControl.m_oLogicDocument=new AscCommonWord.CDocument(this.WordControl.m_oDrawingDocument),this.WordControl.m_oDrawingDocument.m_oLogicDocument=this.WordControl.m_oLogicDocument")
	wordInitEditorFix = []byte("asc_docs_api.prototype.InitEditor=function(){AscFormat.ExecuteNoHistory(function(){this.WordControl.m_oLogicDocument=new AscCommonWord.CDocument(this.WordControl.m_oDrawingDocument)},this,[],!0),this.WordControl.m_oDrawingDocument.m_oLogicDocument=this.WordControl.m_oLogicDocument")

	wordBeforeOpenBug = []byte("function BeforeOpenDocument(){this.InitEditor(),this.DocumentType=2,this.LoadedObjectDS=this.WordControl.m_oLogicDocument.CopyStyle(),g_oIdCounter.Set_Load(!0),AscFonts.IsCheckSymbols=!0}")
	wordBeforeOpenFix = []byte("function BeforeOpenDocument(){AscFormat.ExecuteNoHistory(function(){this.InitEditor(),this.DocumentType=2,this.LoadedObjectDS=this.WordControl.m_oLogicDocument.CopyStyle(),g_oIdCounter.Set_Load(!0),AscFonts.IsCheckSymbols=!0},this,[],!0)}")

	wordOpenFromBinBug = []byte("asc_docs_api.prototype.OpenDocumentFromBin=function(url,gObject){BeforeOpenDocument.call(this);var oBinaryFileReader=new AscCommonWord.BinaryFileReader(this.WordControl.m_oLogicDocument,{});return oBinaryFileReader.Read(gObject)||editor.sendEvent(\"asc_onError\",c_oAscError.ID.MobileUnexpectedCharCount,c_oAscError.Level.Critical),AfterOpenDocument.call(this,oBinaryFileReader.stream.data,oBinaryFileReader.stream.size),!0}")
	wordOpenFromBinFix = []byte("asc_docs_api.prototype.OpenDocumentFromBin=function(url,gObject){return AscFormat.ExecuteNoHistory(function(){BeforeOpenDocument.call(this);var oBinaryFileReader=new AscCommonWord.BinaryFileReader(this.WordControl.m_oLogicDocument,{});return oBinaryFileReader.Read(gObject)||editor.sendEvent(\"asc_onError\",c_oAscError.ID.MobileUnexpectedCharCount,c_oAscError.Level.Critical),AfterOpenDocument.call(this,oBinaryFileReader.stream.data,oBinaryFileReader.stream.size),!0},this,[],!0)}")

	wordOpenFromZipBug = []byte("asc_docs_api.prototype.OpenDocumentFromZip=function(data){BeforeOpenDocument.call(this);let res=this.OpenDocumentFromZipNoInit(data);return AfterOpenDocument.call(this,data,data.length),res}")
	wordOpenFromZipFix = []byte("asc_docs_api.prototype.OpenDocumentFromZip=function(data){return AscFormat.ExecuteNoHistory(function(){BeforeOpenDocument.call(this);let res=this.OpenDocumentFromZipNoInit(data);return AfterOpenDocument.call(this,data,data.length),res},this,[],!0)}")

	slideInitEditorBug = []byte("asc_docs_api.prototype.InitEditor=function(){this.WordControl.m_oLogicDocument=new AscCommonSlide.CPresentation(this.WordControl.m_oDrawingDocument),this.WordControl.m_oDrawingDocument.m_oLogicDocument=this.WordControl.m_oLogicDocument")
	slideInitEditorFix = []byte("asc_docs_api.prototype.InitEditor=function(){AscFormat.ExecuteNoHistory(function(){this.WordControl.m_oLogicDocument=new AscCommonSlide.CPresentation(this.WordControl.m_oDrawingDocument)},this,[],!0),this.WordControl.m_oDrawingDocument.m_oLogicDocument=this.WordControl.m_oLogicDocument")

	slideOpenFromBinBug = []byte("asc_docs_api.prototype.OpenDocumentFromBin=function(url,gObject){this.InitEditor(),this.DocumentType=2;var _loader=new AscCommon.BinaryPPTYLoader;_loader.Api=this,g_oIdCounter.Set_Load(!0),AscFonts.IsCheckSymbols=!0,_loader.Load(gObject,this.WordControl.m_oLogicDocument),this.WordControl.m_oLogicDocument.Set_FastCollaborativeEditing(!0)")
	slideOpenFromBinFix = []byte("asc_docs_api.prototype.OpenDocumentFromBin=function(url,gObject){this.InitEditor(),this.DocumentType=2;var _loader=new AscCommon.BinaryPPTYLoader;_loader.Api=this,g_oIdCounter.Set_Load(!0),AscFonts.IsCheckSymbols=!0,AscFormat.ExecuteNoHistory(function(){_loader.Load(gObject,this.WordControl.m_oLogicDocument)},this,[],!0),this.WordControl.m_oLogicDocument.Set_FastCollaborativeEditing(!0)")

	// Vanilla sdkjs hardcodes the coauthoring JWT to the literal "fghhfgsjdgfjs" and never
	// forwards the real `config.token`:
	//
	//   this.CoAuthoringApi.init(this.User,this.documentId,this.documentCallbackUrl,
	//                            "fghhfgsjdgfjs", ...)
	//
	// DocsCoApi._token is then sent as the `token` field of every coauthoring `auth`
	// packet, so a document server that verifies JWTs rejects every session with
	// "token contains an invalid number of segments" (15 plain characters, not a JWT).
	// The real token is already available: init() reads it from docInfo into
	// this.jwtOpen a few statements later. Prefer it over the passed placeholder.
	//
	// Upstream ONLYOFFICE builds do not have this bug; it is specific to this bundle.
	//
	// The fix replaces the tail of the statement rather than appending to it, so the
	// original anchor no longer matches and patchSDKJSBundle stays idempotent.
	coauthoringTokenBug = []byte("this.jwtOpen=docInfo.get_Token()")
	coauthoringTokenFix = []byte("this.jwtOpen=docInfo.get_Token(),this._token=this.jwtOpen||this._token")
)

// patchSDKJS applies vendor hotfixes to Euro-Office sdkjs load paths. Vanilla 9.3.4
// records undo history while deserializing Editor.bin (CustomXmlManager, CDocProtect, …)
// which throws Write_ToBinary2 under parallel browser load.
func patchSDKJS(outDir string) error {
	patches := []struct {
		path     string
		buggy    []byte
		fixed    []byte
		optional bool
	}{
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all.js"), cellCustomXMLHistoryBug, cellCustomXMLHistoryFix, true},
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all-min.js"), cellCustomXMLHistoryBug, cellCustomXMLHistoryFix, true},
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all-min.js"), cellOpenFromBinNoInitBug, cellOpenFromBinNoInitFix, false},
		// Required: without this the coauthoring auth packet carries a non-JWT placeholder
		// and any document server verifying JWTs rejects every session.
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all-min.js"), coauthoringTokenBug, coauthoringTokenFix, false},

		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordInitEditorBug, wordInitEditorFix, false},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordBeforeOpenBug, wordBeforeOpenFix, false},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordOpenFromBinBug, wordOpenFromBinFix, false},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordOpenFromZipBug, wordOpenFromZipFix, false},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), coauthoringTokenBug, coauthoringTokenFix, false},

		{filepath.Join(outDir, "sdkjs", "slide", "sdk-all-min.js"), slideInitEditorBug, slideInitEditorFix, false},
		{filepath.Join(outDir, "sdkjs", "slide", "sdk-all-min.js"), slideOpenFromBinBug, slideOpenFromBinFix, false},
		{filepath.Join(outDir, "sdkjs", "slide", "sdk-all-min.js"), coauthoringTokenBug, coauthoringTokenFix, false},
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
	// Already applied? Two shapes are possible:
	//   - replacement patches: `fixed` present and `buggy` gone
	//   - append-style patches (fixed extends buggy): `fixed` present at all
	// Without the second case an append-style patch is never recognised as applied and
	// would be applied repeatedly on every run.
	if bytes.Contains(data, fixed) {
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
