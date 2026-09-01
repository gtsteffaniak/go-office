package convert

import (
	"strings"
	"testing"
)

func TestFormatOutputExtension(t *testing.T) {
	if FormatOutputExtension("jpg") != FormatOutputJPG {
		t.Fatalf("jpg = %d want %d", FormatOutputExtension("jpg"), FormatOutputJPG)
	}
	if FormatOutputExtension("png") != FormatOutputPNG {
		t.Fatal("png")
	}
}

func TestBuildConvertTaskXMLDocxToJpg(t *testing.T) {
	xml := buildConvertTaskXML(
		"/in/sample.docx", "/out/thumb.jpg",
		"/fonts", "/themes",
		"docx", "jpg", FormatOutputJPG,
		&Thumbnail{Width: 200, Height: 200, Aspect: 2, First: true},
		"/run/AllFonts.js", "/run/work",
	)
	for _, want := range []string{
		"<m_nFormatFrom>65</m_nFormatFrom>",
		"<m_nFormatTo>1025</m_nFormatTo>",
		"<m_sJsonParams>",
		`&quot;thumbnail&quot;`,
		`&quot;width&quot;:200`,
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("missing %q in:\n%s", want, xml)
		}
	}
}

func TestConvOutputBasename(t *testing.T) {
	if officeConvOutputBasename("jpg", nil) != "output.jpg" {
		t.Fatal("jpg basename")
	}
}

// officeConvOutputBasename mirrors office package helper for convert-only tests.
func officeConvOutputBasename(outputType string, thumb *Thumbnail) string {
	if thumb != nil && !thumb.First {
		return "output.zip"
	}
	switch outputType {
	case "jpeg":
		return "output.jpg"
	default:
		return "output." + outputType
	}
}
