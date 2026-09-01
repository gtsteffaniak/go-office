//go:build linux

package convert_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/testutil"
)

func TestConvertFileDocxToJpg(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
	if !testutil.SampleExists(repo, "sample-files/sample.docx") {
		t.Skip("sample docx missing")
	}

	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.docx")))
	out := filepath.Join(t.TempDir(), "thumb.jpg")

	c, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := c.ConvertFile(ctx, convert.ConvertRequest{
		SourcePath: src,
		DestPath:   out,
		FileType:   "docx",
		OutputType: "jpg",
		Thumbnail:  &convert.Thumbnail{Width: 200, Height: 200, Aspect: 2, First: true},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 100 || !isRasterImage(raw) {
		t.Fatalf("expected image, got %d bytes start %02x %02x", len(raw), raw[0], raw[1])
	}
}

func TestConvertFileXlsxToJpg(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
	if !testutil.SampleExists(repo, "sample-files/sample.xlsx") {
		t.Skip("sample xlsx missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.xlsx")))
	out := filepath.Join(t.TempDir(), "thumb.jpg")
	c, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := c.ConvertFile(ctx, convert.ConvertRequest{
		SourcePath: src,
		DestPath:   out,
		FileType:   "xlsx",
		OutputType: "jpg",
		Thumbnail:  &convert.Thumbnail{Width: 200, Height: 200, Aspect: 2, First: true},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestConvertFileDotToJpg(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
	if !testutil.SampleExists(repo, "sample-files/sample.dot") {
		t.Skip("sample dot missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.dot")))
	out := filepath.Join(t.TempDir(), "thumb.jpg")
	c, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := c.ConvertFile(ctx, convert.ConvertRequest{
		SourcePath: src,
		DestPath:   out,
		FileType:   "dot",
		OutputType: "jpg",
		Thumbnail:  &convert.Thumbnail{Width: 200, Height: 200, Aspect: 2, First: true},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 100 || !isRasterImage(raw) {
		t.Fatalf("expected image, got %d bytes start %02x %02x", len(raw), raw[0], raw[1])
	}
}

func isRasterImage(raw []byte) bool {
	if len(raw) < 4 {
		return false
	}
	if raw[0] == 0xff && raw[1] == 0xd8 {
		return true // JPEG
	}
	return raw[0] == 0x89 && raw[1] == 0x50 && raw[2] == 0x4e && raw[3] == 0x47 // PNG
}
