package convert

import (
	"strings"
	"testing"
)

func TestBuildReverseTaskXMLFromChanges(t *testing.T) {
	runDir := "/cache/key/x2t-run"
	xml := buildReverseTaskXML("/cache/key/Editor.bin", "/cache/key/saved.docx", runDir, "/themes", runDir+"/AllFonts.js", "docx", true, "/cache/key/x2t-save-abc123")
	if !strings.Contains(xml, "<m_sFontDir>"+runDir+"</m_sFontDir>") {
		t.Fatalf("fromChanges font dir must be isolated run dir: %s", xml)
	}
	if !strings.Contains(xml, "<m_sAllFontsPath>"+runDir+"/AllFonts.js</m_sAllFontsPath>") {
		t.Fatalf("fromChanges all fonts must be under run dir: %s", xml)
	}
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
	if !strings.Contains(xml, "<m_sTempDir>/cache/key/x2t-save-") {
		t.Fatalf("fromChanges temp dir must live under document cache: %s", xml)
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

func TestBuildTaskXMLTxt(t *testing.T) {
	xml := buildTaskXML("/samples/sample.txt", "/cache/Editor.bin", "/fonts", "/themes", ".txt", "/run/AllFonts.js", "/run")
	if !strings.Contains(xml, "<m_nFormatFrom>69</m_nFormatFrom>") {
		t.Fatalf("txt formatFrom missing: %s", xml)
	}
	if !strings.Contains(xml, "<m_nCsvTxtEncoding>46</m_nCsvTxtEncoding>") {
		t.Fatalf("txt encoding missing: %s", xml)
	}
	if !strings.Contains(xml, "<m_sAllFontsPath>") {
		t.Fatalf("all fonts path missing: %s", xml)
	}
}

func TestBuildOfficeToOfficeXMLXlsxToCSV(t *testing.T) {
	xml := buildOfficeToOfficeXML("/cache/saved.xlsx", "/cache/saved.csv", "/fonts", "/themes", "/run/AllFonts.js", "xlsx", "csv", "/tmp/x2t")
	if !strings.Contains(xml, "<m_nFormatFrom>257</m_nFormatFrom>") {
		t.Fatalf("xlsx formatFrom missing: %s", xml)
	}
	if !strings.Contains(xml, "<m_nFormatTo>260</m_nFormatTo>") {
		t.Fatalf("csv formatTo missing: %s", xml)
	}
	if strings.Contains(xml, "<m_bFromChanges>true</m_bFromChanges>") {
		t.Fatal("xlsx→csv must not set fromChanges")
	}
	if !strings.Contains(xml, "<m_sAllFontsPath>") {
		t.Fatalf("all fonts path missing: %s", xml)
	}
}
