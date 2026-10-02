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

	// CustomXmlManager's constructor registers itself in the global object table:
	//
	//   function CustomXmlManager(document){...,AscCommon.g_oTableId.Add(this,this.Id)}
	//
	// During document load that Add() runs with undo history enabled, so the history tries
	// to serialise a CustomXmlManager before it is fully constructed and sdkjs throws:
	//
	//   TypeError: this.NewClass.Write_ToBinary2 is not a function
	//     at CChangesTableIdAdd.WriteToBinary (sdk-all-min.js)
	//     at UndoRedoItemSerializable.SerializeInner (sdk-all.js)
	//     at Workbook._SerializeHistoryItem2
	//     at CHistory.Refresh_SpreadsheetChanges
	//     at CHistory.Add
	//     at CTableId.Add
	//     at new CustomXmlManager
	//
	// The spreadsheet then never finishes loading: the cell-name box stays disabled and
	// every cell-editing test times out. Wrapping the registration in ExecuteNoHistory keeps
	// it out of the undo stack, which is correct because opening a document is not an
	// undoable user action.
	//
	// Note: the older cellCustomXMLHistory fix above wraps the *caller* in sdk-all.js. That
	// is insufficient — the failing frame is inside the constructor, which is a separate
	// site in the same bundle.
	cellCustomXMLCtorBug = []byte("function CustomXmlManager(document){this.Id=AscCommon.g_oIdCounter.Get_NewId(),this.document=document,this.xml=[],this.m_arrXmlById={},AscCommon.g_oTableId.Add(this,this.Id)}")
	cellCustomXMLCtorFix = []byte("function CustomXmlManager(document){this.Id=AscCommon.g_oIdCounter.Get_NewId(),this.document=document,this.xml=[],this.m_arrXmlById={},AscFormat.ExecuteNoHistory(function(){AscCommon.g_oTableId.Add(this,this.Id)},this,[],!0)}")

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
	// The guard must test for *presence*, not truthiness. document.token is legitimately the
	// empty string when the server runs with JWT verification disabled, and a falsy check
	// (`this.jwtOpen||this._token`) then keeps the placeholder on the wire — which is exactly
	// what made the first version of this patch silently ineffective.
	//
	// Upstream ONLYOFFICE builds do not have this bug; it is specific to this bundle.
	//
	// The fix replaces the tail of the statement rather than appending to it, so the
	// original anchor no longer matches and patchSDKJSBundle stays idempotent.
	coauthoringTokenBug = []byte("this.jwtOpen=docInfo.get_Token()")
	coauthoringTokenFix = []byte("this.jwtOpen=docInfo.get_Token(),this._token=null==this.jwtOpen?this._token:this.jwtOpen")

	// Superseded first revision of the fix above. It used a truthiness guard
	// (`this.jwtOpen||this._token`), which still yielded the hardcoded placeholder whenever
	// document.token was the empty string — the normal case with JWT verification disabled.
	// Trees patched by an earlier build contain this text instead of the buggy anchor, so it
	// must be upgraded explicitly rather than reported as an unsupported bundle.
	coauthoringTokenFixSuperseded = []byte("this.jwtOpen=docInfo.get_Token(),this._token=this.jwtOpen||this._token")
)

// ApplyPatches applies the sdkjs hotfixes to an existing asset tree.
//
// Patch application is part of *preparing* an asset tree, not part of fetching it, so it must
// run for any tree go-office is about to serve — including one that was already present on
// disk. DiscoverAssets previously returned such a tree without patching it, which meant a
// developer or CI job with a pre-populated assets/ directory silently ran an unpatched sdkjs
// (spreadsheet load crash, hardcoded coauthoring token) while a fresh fetch of the same
// version was fine. ApplyPatches is idempotent and cheap, so callers may invoke it freely.
func ApplyPatches(outDir string) error {
	return patchSDKJS(outDir)
}

// patchSDKJS applies vendor hotfixes to Euro-Office sdkjs load paths. Vanilla 9.3.4
// records undo history while deserializing Editor.bin (CustomXmlManager, CDocProtect, …)
// which throws Write_ToBinary2 under parallel browser load.
func patchSDKJS(outDir string) error {
	patches := []struct {
		path     string
		buggy    []byte
		fixed    []byte
		optional bool
		// superseded holds earlier revisions of `fixed` that may already be present in an
		// asset tree patched by a previous build. They are rewritten to the current `fixed`
		// so an upgrade repairs stale output instead of failing as an unsupported bundle.
		superseded [][]byte
	}{
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all.js"), cellCustomXMLHistoryBug, cellCustomXMLHistoryFix, true, nil},
		// Required: this is the site that actually throws. Note sdk-all-min.js has no
		// CustomXmlManager constructor, so this anchor is only expected in sdk-all.js.
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all.js"), cellCustomXMLCtorBug, cellCustomXMLCtorFix, false, nil},
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all-min.js"), cellCustomXMLHistoryBug, cellCustomXMLHistoryFix, true, nil},
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all-min.js"), cellOpenFromBinNoInitBug, cellOpenFromBinNoInitFix, false, nil},
		// Required: without this the coauthoring auth packet carries a non-JWT placeholder
		// and any document server verifying JWTs rejects every session.
		{filepath.Join(outDir, "sdkjs", "cell", "sdk-all-min.js"), coauthoringTokenBug, coauthoringTokenFix, false, [][]byte{coauthoringTokenFixSuperseded}},

		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordInitEditorBug, wordInitEditorFix, false, nil},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordBeforeOpenBug, wordBeforeOpenFix, false, nil},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordOpenFromBinBug, wordOpenFromBinFix, false, nil},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), wordOpenFromZipBug, wordOpenFromZipFix, false, nil},
		{filepath.Join(outDir, "sdkjs", "word", "sdk-all-min.js"), coauthoringTokenBug, coauthoringTokenFix, false, [][]byte{coauthoringTokenFixSuperseded}},

		{filepath.Join(outDir, "sdkjs", "slide", "sdk-all-min.js"), slideInitEditorBug, slideInitEditorFix, false, nil},
		{filepath.Join(outDir, "sdkjs", "slide", "sdk-all-min.js"), slideOpenFromBinBug, slideOpenFromBinFix, false, nil},
		{filepath.Join(outDir, "sdkjs", "slide", "sdk-all-min.js"), coauthoringTokenBug, coauthoringTokenFix, false, [][]byte{coauthoringTokenFixSuperseded}},
	}
	for _, patch := range patches {
		if err := patchSDKJSBundle(patch.path, patch.buggy, patch.fixed, patch.optional, patch.superseded...); err != nil {
			return err
		}
	}
	return nil
}

func patchSDKJSBundle(path string, buggy, fixed []byte, optional bool, superseded ...[]byte) error {
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
	// Upgrade a stale revision of this same patch written by an earlier build.
	for _, old := range superseded {
		if len(old) == 0 || !bytes.Contains(data, old) {
			continue
		}
		data = bytes.Replace(data, old, fixed, 1)
		return os.WriteFile(path, data, 0o644)
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
