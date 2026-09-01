package office_test

import (
	"encoding/json"
	"testing"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestParseConverterRequestFileBrowserCasing(t *testing.T) {
	raw := []byte(`{
		"Filetype": ".docx",
		"key": "abc123",
		"outputType": "jpg",
		"title": "report.docx",
		"url": "http://127.0.0.1/files/report.docx",
		"thumbnail": {"width": 200, "height": 200}
	}`)
	req, err := office.ParseConverterRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.FileType != "docx" || req.Key != "abc123" || req.OutputType != "jpg" {
		t.Fatalf("unexpected req: %+v", req)
	}
	if req.Thumbnail == nil || req.Thumbnail.Width != 200 {
		t.Fatalf("thumbnail: %+v", req.Thumbnail)
	}
}

func TestConvCacheDirName(t *testing.T) {
	got := office.ConvCacheDirName("my/key", "jpg")
	if got != "conv_my_key_jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestConvFileURL(t *testing.T) {
	u := office.ConvFileURL("http://localhost:8080", "", "conv_key_jpg", "output.jpg", "a.docx")
	want := "http://localhost:8080/cache/files/conv_key_jpg/output.jpg?filename=a.docx"
	if u != want {
		t.Fatalf("got %q want %q", u, want)
	}
}

func TestConverterResponseJSON(t *testing.T) {
	res := office.ConverterResponse{
		EndConvert: true,
		FileType:   "jpg",
		FileURL:    "http://localhost/cache/files/conv_k_jpg/output.jpg",
		Percent:    100,
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid json: %s", raw)
	}
}
