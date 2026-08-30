package config

// EditorRequest is the host-provided metadata needed to open a document.
type EditorRequest struct {
	DocumentKey  string
	Title        string
	FileType     string
	DocumentType string // word, cell, slide — inferred from FileType if empty
	DocumentURL  string
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
		"document": map[string]any{
			"fileType": req.FileType,
			"key":      req.DocumentKey,
			"title":    req.Title,
			"url":      req.DocumentURL,
			"permissions": map[string]any{
				"edit":     req.Permissions.Edit,
				"download": req.Permissions.Download,
				"print":    req.Permissions.Print,
			},
		},
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
			"lang": req.Lang,
			"mode": mode,
		},
	}
	if token != "" {
		out["token"] = token
	}
	return out
}

func inferDocumentType(fileType string) string {
	switch fileType {
	case "xls", "xlsx", "xlsm", "xlsb", "ods", "csv":
		return "cell"
	case "ppt", "pptx", "pptm", "odp":
		return "slide"
	default:
		return "word"
	}
}
