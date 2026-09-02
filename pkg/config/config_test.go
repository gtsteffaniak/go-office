package config_test

import (
	"testing"

	"github.com/quantumx-apps/go-office/pkg/config"
)

func TestBuildWordDocument(t *testing.T) {
	cfg := config.Build(config.EditorRequest{
		DocumentKey: "abc",
		Title:       "test.docx",
		FileType:    "docx",
		DocumentURL: "http://host/file.docx",
		CallbackURL: "http://host/callback",
		UserID:      "1",
		UserName:    "Alice",
		Mode:        "edit",
		Lang:        "en",
		Theme:       "dark",
		Permissions: config.Permissions{Edit: "edit", Download: true, Print: true},
	}, "signed")

	if cfg["token"] != "signed" {
		t.Fatalf("token = %v", cfg["token"])
	}
	if cfg["documentType"] != "word" {
		t.Fatalf("documentType = %v", cfg["documentType"])
	}
	doc, ok := cfg["document"].(map[string]any)
	if !ok {
		t.Fatalf("document type = %T", cfg["document"])
	}
	if doc["key"] != "abc" {
		t.Fatalf("key = %v", doc["key"])
	}
	perms, ok := doc["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions type = %T", doc["permissions"])
	}
	if perms["edit"] != true {
		t.Fatalf("permissions.edit must be boolean true, got %v (%T)", perms["edit"], perms["edit"])
	}
}

func TestInferDocumentType(t *testing.T) {
	cases := map[string]string{
		"xlsx": "cell",
		"pptx": "slide",
		"docx": "word",
		"pdf":  "pdf",
	}
	for ext, want := range cases {
		got := config.Build(config.EditorRequest{FileType: ext}, "")["documentType"]
		if got != want {
			t.Fatalf("%s: got %v want %s", ext, got, want)
		}
	}
}

func TestPDFConfigSkipsCommonBootstrap(t *testing.T) {
	cfg := config.Build(config.EditorRequest{FileType: "pdf"}, "")
	doc, ok := cfg["document"].(map[string]any)
	if !ok {
		t.Fatalf("document type = %T", cfg["document"])
	}
	if doc["isForm"] != false {
		t.Fatalf("isForm = %v, want false", doc["isForm"])
	}
	if cfg["documentType"] != "pdf" {
		t.Fatalf("documentType = %v", cfg["documentType"])
	}
}

func TestViewModeDisablesEditPermission(t *testing.T) {
	cfg := config.Build(config.EditorRequest{FileType: "docx", Mode: "view", Permissions: config.Permissions{Edit: "view"}}, "")
	doc, ok := cfg["document"].(map[string]any)
	if !ok {
		t.Fatalf("document type = %T", cfg["document"])
	}
	perms, ok := doc["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions type = %T", doc["permissions"])
	}
	if perms["edit"] != false {
		t.Fatalf("view mode permissions.edit = %v", perms["edit"])
	}
}
