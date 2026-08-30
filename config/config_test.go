package config_test

import (
	"testing"

	"github.com/quantumx-apps/go-office/config"
)

func TestBuildWordDocument(t *testing.T) {
	cfg := config.Build(config.EditorRequest{
		DocumentKey:  "abc",
		Title:        "test.docx",
		FileType:     "docx",
		DocumentURL:  "http://host/file.docx",
		CallbackURL:  "http://host/callback",
		UserID:       "1",
		UserName:     "Alice",
		Mode:         "edit",
		Lang:         "en",
		Theme:        "dark",
		Permissions:  config.Permissions{Edit: "edit", Download: true, Print: true},
	}, "signed")

	if cfg["token"] != "signed" {
		t.Fatalf("token = %v", cfg["token"])
	}
	if cfg["documentType"] != "word" {
		t.Fatalf("documentType = %v", cfg["documentType"])
	}
	doc := cfg["document"].(map[string]any)
	if doc["key"] != "abc" {
		t.Fatalf("key = %v", doc["key"])
	}
}

func TestInferDocumentType(t *testing.T) {
	cases := map[string]string{
		"xlsx": "cell",
		"pptx": "slide",
		"docx": "word",
	}
	for ext, want := range cases {
		got := config.Build(config.EditorRequest{FileType: ext}, "")["documentType"]
		if got != want {
			t.Fatalf("%s: got %v want %s", ext, got, want)
		}
	}
}
