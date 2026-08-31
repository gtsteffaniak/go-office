package config

import "strings"

// EditorRequest is the host-provided metadata needed to open a document.
type EditorRequest struct {
	DocumentKey  string
	Title        string
	FileType     string
	DocumentType string // word, cell, slide, pdf — inferred from FileType if empty
	DocumentURL  string
	StoragePath  string // host VFS path for Save; empty when only URL is available
	CallbackURL  string
	UserID       string
	UserName     string
	Mode         string // edit or view
	Lang         string
	Theme        string // dark or light
	Permissions  Permissions
}

// Permissions mirror the ONLYOFFICE editor config permissions block.
type Permissions struct {
	Edit     string // "edit" or "view"
	Download bool
	Print    bool
}

// Build produces a map suitable for @onlyoffice/document-editor-vue.
func Build(req EditorRequest, token string) map[string]any {
	docType := req.DocumentType
	if docType == "" {
		docType = inferDocumentType(req.FileType)
	}
	mode := req.Mode
	if mode == "" {
		mode = req.Permissions.Edit
	}
	out := map[string]any{
		"document":     buildDocument(req),
		"documentType": docType,
		"editorConfig": map[string]any{
			"callbackUrl": req.CallbackURL,
			"user": map[string]any{
				"id":   req.UserID,
				"name": req.UserName,
			},
			"customization": map[string]any{
				"autosave":  true,
				"forcesave": true,
				"uiTheme":   req.Theme,
			},
			"coEditing": map[string]any{
				"mode":   "fast",
				"change": false,
			},
			"lang": req.Lang,
			"mode": mode,
		},
	}
	if token != "" {
		out["token"] = token
	}
	return out
}

func buildDocument(req EditorRequest) map[string]any {
	doc := map[string]any{
		"fileType": req.FileType,
		"key":      req.DocumentKey,
		"title":    req.Title,
		"url":      req.DocumentURL,
		"permissions": map[string]any{
			"edit":     permissionEdit(req),
			"download": req.Permissions.Download,
			"print":    req.Permissions.Print,
		},
	}
	// Skip the common/ bootstrap iframe (it hangs when served as /common/ without index.html).
	if strings.EqualFold(req.FileType, "pdf") {
		doc["isForm"] = false
	}
	return doc
}

func inferDocumentType(fileType string) string {
	switch fileType {
	case "xls", "xlsx", "xlsm", "xlsb", "ods", "csv":
		return "cell"
	case "ppt", "pptx", "pptm", "odp":
		return "slide"
	case "pdf":
		return "pdf"
	default:
		return "word"
	}
}

func permissionEdit(req EditorRequest) bool {
	if strings.EqualFold(req.Mode, "view") || strings.EqualFold(req.Permissions.Edit, "view") {
		return false
	}
	return true
}
