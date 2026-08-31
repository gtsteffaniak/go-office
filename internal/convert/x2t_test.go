package convert

import (
	"strings"
	"testing"
)

func TestBuildReverseTaskXMLFromChanges(t *testing.T) {
	xml := buildReverseTaskXML("/cache/key/Editor.bin", "/cache/key/saved.docx", "/converter/bin", "/themes", "/converter/bin/AllFonts.js", "docx", true, "/tmp/x2t")
	if !strings.Contains(xml, "<m_bFromChanges>true</m_bFromChanges>") {
		t.Fatalf("fromChanges flag missing: %s", xml)
	}
	if !strings.Contains(xml, "<m_sAllFontsPath>") {
		t.Fatalf("all fonts path missing: %s", xml)
	}
	if !strings.Contains(xml, "<m_nFormatTo>65</m_nFormatTo>") {
		t.Fatalf("formatTo missing: %s", xml)
	}
	if strings.Contains(xml, "<m_nFormatFrom>") {
		t.Fatalf("formatFrom should be omitted for fromChanges saves: %s", xml)
	}
	if !strings.Contains(xml, "<m_nDoctParams>1</m_nDoctParams>") {
		t.Fatalf("doct params missing: %s", xml)
	}
}

func TestBuildReverseTaskXMLDirect(t *testing.T) {
	xml := buildReverseTaskXML("/cache/key/Editor.bin", "/cache/key/saved.xlsx", "/converter/bin", "/themes", "/converter/bin/AllFonts.js", "xlsx", false, "")
	if !strings.Contains(xml, "<m_bFromChanges>false</m_bFromChanges>") {
		t.Fatalf("expected explicit fromChanges false: %s", xml)
	}
	if !strings.Contains(xml, "<m_nFormatFrom>8194</m_nFormatFrom>") {
		t.Fatalf("spreadsheet canvas format missing: %s", xml)
	}
}

func TestBuildReverseTaskXMLCSV(t *testing.T) {
	xml := buildReverseTaskXML("/cache/key/Editor.bin", "/cache/key/saved.csv", "/converter/bin", "/themes", "/converter/bin/AllFonts.js", "csv", true, "/tmp/x2t")
	if !strings.Contains(xml, "<m_nCsvDelimiter>4</m_nCsvDelimiter>") {
		t.Fatalf("csv delimiter missing: %s", xml)
	}
	if !strings.Contains(xml, "<m_sJsonParams>") {
		t.Fatalf("spreadsheet json params missing: %s", xml)
	}
}

func TestCSVNeedsXlsxBridge(t *testing.T) {
	if !csvNeedsXlsxBridge("csv") || !csvNeedsXlsxBridge("CSV") {
		t.Fatal("csv must use the xlsx apply-changes path")
	}
	if csvNeedsXlsxBridge("xlsx") || csvNeedsXlsxBridge("docx") {
		t.Fatal("xlsx/docx should not use the csv bridge")
	}
}

func TestBuildOfficeToOfficeXMLXlsxToCSV(t *testing.T) {
	xml := buildOfficeToOfficeXML("/cache/saved.xlsx", "/cache/saved.csv", "/fonts", "/themes", "xlsx", "csv", "/tmp/x2t")
	if !strings.Contains(xml, "<m_nFormatFrom>257</m_nFormatFrom>") {
		t.Fatalf("xlsx formatFrom missing: %s", xml)
	}
	if !strings.Contains(xml, "<m_nFormatTo>260</m_nFormatTo>") {
		t.Fatalf("csv formatTo missing: %s", xml)
	}
	if strings.Contains(xml, "<m_bFromChanges>true</m_bFromChanges>") {
		t.Fatal("xlsx→csv must not set fromChanges")
	}
}
