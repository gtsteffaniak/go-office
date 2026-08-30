package convert

import (
	"strings"
	"testing"
)

func TestBuildReverseTaskXMLFromChanges(t *testing.T) {
	xml := buildReverseTaskXML("/cache/key/Editor.bin", "/cache/key/saved.docx", "/fonts", "/themes", "docx", true)
	if !strings.Contains(xml, "<m_bFromChanges>true</m_bFromChanges>") {
		t.Fatalf("fromChanges flag missing: %s", xml)
	}
	if !strings.Contains(xml, "<m_nFormatTo>65</m_nFormatTo>") {
		t.Fatalf("formatTo missing: %s", xml)
	}
}

func TestBuildReverseTaskXMLDirect(t *testing.T) {
	xml := buildReverseTaskXML("/cache/key/Editor.bin", "/cache/key/saved.xlsx", "/fonts", "/themes", "xlsx", false)
	if strings.Contains(xml, "<m_bFromChanges>") {
		t.Fatalf("unexpected fromChanges in direct convert: %s", xml)
	}
	if !strings.Contains(xml, "<m_nFormatFrom>8194</m_nFormatFrom>") {
		t.Fatalf("spreadsheet canvas format missing: %s", xml)
	}
}
