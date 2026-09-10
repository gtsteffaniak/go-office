//go:build linux

package convert_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/testutil"
)

func TestSaveChangesCSVRoundTrip(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	work := testutil.NewWorkspace(t)
	csvPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.csv")))

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}

	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, csvPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}

	// Without changes, reverse conversion should succeed.
	outPath := filepath.Join(cacheDir, "saved.csv")
	err = conv.FromEditorBin(ctx, cacheDir, outPath, "csv")
	if err != nil {
		t.Fatalf("from editor bin: %v", err)
	}
	st, statErr := os.Stat(outPath)
	if statErr != nil || st.Size() == 0 {
		t.Fatalf("saved.csv missing or empty")
	}

	fixture := filepath.Join(repo, "internal", "convert", "testdata", "csv_changes0.json")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Skipf("csv change fixture missing: %v", err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	err = os.MkdirAll(changesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), raw, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	var entries []string
	err = json.Unmarshal(raw, &entries)
	if err != nil || len(entries) == 0 {
		t.Fatalf("invalid fixture: %v", err)
	}

	outWithChanges := filepath.Join(cacheDir, "saved-with-changes.csv")
	_, err = conv.SaveChanges(ctx, cacheDir, outWithChanges, "csv")
	if err != nil {
		t.Fatalf("save changes: %v", err)
	}
	body, err := os.ReadFile(outWithChanges)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("saved-with-changes.csv is empty")
	}
	// Captured spreadsheet blobs are session-specific; the regression this
	// catches is x2t crashing (exit 86 / "$ is not defined") while applying
	// changes0.json. The original sheet content must still round-trip.
	if !strings.Contains(string(body), "Customer Id") {
		t.Fatalf("csv lost original content after SaveChanges: %q", truncate(body, 200))
	}
	if st, err := os.Stat(filepath.Join(cacheDir, "changes-applied.xlsx")); err != nil || st.Size() == 0 {
		t.Fatalf("csv save must write an xlsx intermediate so x2t can apply_changes: %v", err)
	}
}

func TestSaveChangesTxtRoundTrip(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.txt") {
		t.Skip("sample txt missing")
	}

	work := testutil.NewWorkspace(t)
	txtPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.txt")))

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}

	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, txtPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(cacheDir, "saved.txt")
	err = conv.FromEditorBin(ctx, cacheDir, outPath, "txt")
	if err != nil {
		t.Fatalf("from editor bin: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("saved.txt is empty")
	}
	if !strings.Contains(string(body), "Lorem ipsum") {
		t.Fatalf("txt lost original content after round-trip: %q", truncate(body, 200))
	}
}

func TestSaveChangesRTFRoundTrip(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.rtf") {
		t.Skip("sample rtf missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.rtf")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, src, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	err = os.MkdirAll(changesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["rtf-change"]`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(cacheDir, "saved.rtf")
	_, err = conv.SaveChanges(ctx, cacheDir, outPath, "rtf")
	if err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(cacheDir, "changes-applied.docx")); statErr != nil {
		t.Fatal("rtf save must write docx intermediate before docx→rtf conversion")
	}
	body, err := os.ReadFile(outPath)
	if err != nil || len(body) == 0 {
		t.Fatalf("saved.rtf missing or empty: %v", err)
	}
	if !strings.Contains(convert.RTFPlainText(body), "SYSTEM BRIEF") {
		t.Fatalf("saved.rtf missing document text: %q", truncate(body, 200))
	}
	if strings.Contains(string(body), "&amp;") || strings.Contains(string(body), "&gt;") {
		t.Fatalf("saved.rtf leaked XML entities: %q", truncate(body, 200))
	}
	if !strings.Contains(convert.RTFPlainText(body), "& DAILY") {
		t.Fatalf("saved.rtf missing decoded ampersand in title: %q", truncate(body, 200))
	}
}

func TestSaveChangesRTFFromCorruptedSource(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.rtf") {
		t.Skip("sample rtf missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.rtf")))
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := strings.ReplaceAll(string(raw), "SYSTEM BRIEF & DAILY", "SYSTEM BRIEF &amp; DAILY")
	corrupt = strings.ReplaceAll(corrupt, "> Reminder", "&gt; Reminder")
	if err = os.WriteFile(src, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, src, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	err = os.MkdirAll(changesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["rtf-change"]`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(cacheDir, "saved.rtf")
	_, err = conv.SaveChanges(ctx, cacheDir, outPath, "rtf")
	if err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil || len(body) == 0 {
		t.Fatalf("saved.rtf missing or empty: %v", err)
	}
	if strings.Contains(string(body), "&amp;") || strings.Contains(string(body), "&gt;") {
		t.Fatalf("saved.rtf should repair corrupted entity literals: %q", truncate(body, 200))
	}
	plain := convert.RTFPlainText(body)
	if !strings.Contains(plain, "SYSTEM BRIEF") || !strings.Contains(plain, "DAILY") {
		t.Fatalf("saved.rtf missing repaired title text: %q", truncate(body, 200))
	}
}

func TestWriteRTFFromDocxPlainText(t *testing.T) {
	repo := testutil.RepoRoot(t)
	if !testutil.SampleExists(repo, "sample-files/sample.docx") {
		t.Skip("sample docx missing")
	}
	work := testutil.NewWorkspace(t)
	docxPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.docx")))
	outPath := filepath.Join(t.TempDir(), "plain.rtf")
	err := convert.WriteRTFFromDocxPlainText(docxPath, outPath)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 50_000 {
		t.Fatalf("plain rtf too large: %d bytes", len(body))
	}
	text := string(body)
	if !strings.Contains(text, "Demonstration of DOCX") {
		t.Fatalf("plain rtf missing text: %q", truncate(body, 200))
	}
	if strings.Contains(text, "\\u") {
		t.Fatalf("plain rtf should not use unicode escapes: %q", truncate(body, 200))
	}
}

func TestSaveChangesODSRoundTrip(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.ods") {
		t.Skip("sample ods missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.ods")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, src, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(cacheDir, "saved.ods")
	_, err = conv.SaveChanges(ctx, cacheDir, outPath, "ods")
	if err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil || len(body) == 0 {
		t.Fatalf("saved.ods missing or empty: %v", err)
	}
	if !zipEntryContains(outPath, "content.xml", "DD37Cf93aecA6Dc") {
		body, _ := os.ReadFile(outPath)
		t.Fatalf("ods lost original B2 content after SaveChanges: %q", truncate(body, 200))
	}
}

func TestSaveChangesODTRoundTrip(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.odt") {
		t.Skip("sample odt missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.odt")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, src, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	err = os.MkdirAll(changesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["odt-change"]`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(cacheDir, "saved.odt")
	_, err = conv.SaveChanges(ctx, cacheDir, outPath, "odt")
	if err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(cacheDir, "changes-applied.docx")); statErr == nil {
		t.Fatal("odt save must use direct reverse path, not docx bridge step 2")
	}
	body, err := os.ReadFile(outPath)
	if err != nil || len(body) == 0 {
		t.Fatalf("saved.odt missing or empty: %v", err)
	}
}

func TestSaveChangesPPTRoundTrip(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.ppt") {
		t.Skip("sample ppt missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.ppt")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, src, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	err = os.MkdirAll(changesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["ppt-change"]`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(cacheDir, "saved.ppt")
	_, err = conv.SaveChanges(ctx, cacheDir, outPath, "ppt")
	if err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(cacheDir, "changes-applied.pptx")); statErr != nil {
		t.Fatal("ppt save must write pptx intermediate before pptx→ppt conversion")
	}
	body, err := os.ReadFile(outPath)
	if err != nil || len(body) == 0 {
		t.Fatalf("saved.ppt missing or empty: %v", err)
	}
	slideXML, err := convert.OOXMLPart(body, "ppt/slides/slide1.xml")
	if err != nil {
		t.Fatalf("saved.ppt missing slide1.xml (OOXML fallback expected): %v", err)
	}
	if !strings.Contains(string(slideXML), "My Presentation") {
		t.Fatalf("saved.ppt slide1.xml missing title text: %q", truncate(slideXML, 200))
	}
}

func TestSaveReopenTxtAfterSave(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.txt") {
		t.Skip("sample txt missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.txt")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, src, cacheDir); err != nil {
		t.Fatal(err)
	}
	savedPath := filepath.Join(cacheDir, "saved.txt")
	if err = conv.FromEditorBin(ctx, cacheDir, savedPath, "txt"); err != nil {
		t.Fatalf("FromEditorBin: %v", err)
	}
	reopenDir := t.TempDir()
	if err = conv.ToEditorBin(ctx, savedPath, reopenDir); err != nil {
		t.Fatalf("ToEditorBin saved.txt: %v", err)
	}
	outPath := filepath.Join(reopenDir, "reopened.txt")
	if err = conv.FromEditorBin(ctx, reopenDir, outPath, "txt"); err != nil {
		t.Fatalf("FromEditorBin after reopen: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil || len(body) == 0 {
		t.Fatalf("reopened.txt missing or empty: %v", err)
	}
	if !strings.Contains(string(body), "Sample-Files.com") {
		t.Fatalf("txt reopen lost content: %q", truncate(body, 200))
	}
}

func TestSaveReopenRTFAfterSave(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.rtf") {
		t.Skip("sample rtf missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.rtf")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, src, cacheDir); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	if err = os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["rtf-change"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	savedPath := filepath.Join(cacheDir, "saved.rtf")
	if _, err = conv.SaveChanges(ctx, cacheDir, savedPath, "rtf"); err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	savedBody, err := os.ReadFile(savedPath)
	if err != nil || !strings.Contains(convert.RTFPlainText(savedBody), "SYSTEM BRIEF") {
		t.Fatalf("saved.rtf missing plain text before reopen: %v", err)
	}
	if strings.Contains(string(savedBody), "&amp;") || strings.Contains(string(savedBody), "&gt;") {
		t.Fatalf("saved.rtf leaked XML entities before reopen: %q", truncate(savedBody, 200))
	}
	reopenDir := t.TempDir()
	if err = conv.ToEditorBin(ctx, savedPath, reopenDir); err != nil {
		t.Fatalf("ToEditorBin saved.rtf: %v", err)
	}
	st, err := os.Stat(filepath.Join(reopenDir, "Editor.bin"))
	if err != nil || st.Size() == 0 {
		t.Fatalf("reopen Editor.bin missing or empty: %v", err)
	}
}

func TestSaveReopenPPTAfterSave(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.ppt") {
		t.Skip("sample ppt missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.ppt")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, src, cacheDir); err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	if err = os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["ppt-change"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	savedPath := filepath.Join(cacheDir, "saved.ppt")
	if _, err = conv.SaveChanges(ctx, cacheDir, savedPath, "ppt"); err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	if !zipEntryContains(savedPath, "ppt/slides/slide1.xml", "My Presentation") {
		t.Fatal("saved.ppt missing slide title before reopen")
	}
	reopenDir := t.TempDir()
	if err = conv.ToEditorBin(ctx, savedPath, reopenDir); err != nil {
		t.Fatalf("ToEditorBin saved.ppt: %v", err)
	}
	outPath := filepath.Join(reopenDir, "reopened.pptx")
	if err = conv.FromEditorBin(ctx, reopenDir, outPath, "pptx"); err != nil {
		t.Fatalf("FromEditorBin pptx after reopen: %v", err)
	}
	slideXML, err := convert.OOXMLPart(mustReadFile(t, outPath), "ppt/slides/slide1.xml")
	if err != nil {
		t.Fatalf("reopened.pptx missing slide1.xml: %v", err)
	}
	if !strings.Contains(string(slideXML), "My Presentation") {
		t.Fatalf("ppt reopen lost slide title: %q", truncate(slideXML, 200))
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSaveChangesDocOOXMLFallback(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.doc") {
		t.Skip("sample doc missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.doc")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, src, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	err = os.MkdirAll(changesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["doc-change"]`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(cacheDir, "saved.doc")
	_, err = conv.SaveChanges(ctx, cacheDir, outPath, "doc")
	if err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	intermediate := filepath.Join(cacheDir, "changes-applied.docx")
	intermediateBody, err := os.ReadFile(intermediate)
	if err != nil {
		t.Fatal(err)
	}
	outBody, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(outBody) == 0 {
		t.Fatal("saved.doc is empty")
	}
	if string(outBody) != string(intermediateBody) {
		t.Fatal("doc save should fall back to OOXML bytes when x2t cannot write binary Word")
	}
}

func TestSaveChangesXlsOOXMLFallback(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.xls") {
		t.Skip("sample xls missing")
	}
	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.xls")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, src, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	err = os.MkdirAll(changesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["xls-change"]`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(cacheDir, "saved.xls")
	_, err = conv.SaveChanges(ctx, cacheDir, outPath, "xls")
	if err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	intermediate := filepath.Join(cacheDir, "changes-applied.xlsx")
	intermediateBody, err := os.ReadFile(intermediate)
	if err != nil {
		t.Fatal(err)
	}
	outBody, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(outBody) == 0 {
		t.Fatal("saved.xls is empty")
	}
	if string(outBody) != string(intermediateBody) {
		t.Fatal("xls save should fall back to OOXML bytes when x2t cannot write binary Excel")
	}
}

func TestSaveChangesXlsConcurrentPlaywrightLoad(t *testing.T) {
	// Regression for Playwright post-save-stability sample.xls under parallel workers.
	// Office-to-office x2t must use isolated run dirs (same as reverse apply_changes).
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.xls") {
		t.Skip("sample xls missing")
	}

	const (
		workers      = 10
		convertLimit = 6
	)

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: convertLimit})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < workers; i++ {
		t.Run(fmt.Sprintf("worker-%02d", i), func(t *testing.T) {
			t.Parallel()

			work := testutil.NewWorkspace(t)
			xlsPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.xls")))
			cacheDir := t.TempDir()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			if err := conv.ToEditorBin(ctx, xlsPath, cacheDir); err != nil {
				t.Fatalf("ToEditorBin: %v", err)
			}
			changesDir := filepath.Join(cacheDir, "changes")
			if err := os.MkdirAll(changesDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["xls-change"]`), 0o644); err != nil {
				t.Fatal(err)
			}

			outPath := filepath.Join(cacheDir, "saved.xls")
			if _, err := conv.SaveChanges(ctx, cacheDir, outPath, "xls"); err != nil {
				t.Fatalf("SaveChanges: %v", err)
			}
			intermediate, err := os.ReadFile(filepath.Join(cacheDir, "changes-applied.xlsx"))
			if err != nil {
				t.Fatal(err)
			}
			outBody, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatal(err)
			}
			if len(outBody) == 0 {
				t.Fatal("saved.xls is empty")
			}
			if string(outBody) != string(intermediate) {
				t.Fatal("xls save should persist OOXML fallback bytes under concurrent load")
			}
		})
	}
}

func TestConvertOfficeDocxToRTF(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.docx") {
		t.Skip("sample docx missing")
	}
	work := testutil.NewWorkspace(t)
	docxPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.docx")))
	outPath := filepath.Join(t.TempDir(), "saved.rtf")
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = convert.ExportConvertOfficeInner(conv, ctx, docxPath, outPath, "docx", "rtf", "")
	if err != nil {
		t.Fatalf("docx→rtf: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil || len(body) == 0 {
		t.Fatalf("saved.rtf missing or empty: %v", err)
	}
	t.Logf("docx→rtf bytes=%d", len(body))
}

func TestConvertOfficeDocxToRTFPreservesFormatting(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.docx") {
		t.Skip("sample docx missing")
	}
	work := testutil.NewWorkspace(t)
	docxPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.docx")))
	outPath := filepath.Join(t.TempDir(), "saved.rtf")
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = convert.ExportConvertOfficeInner(conv, ctx, docxPath, outPath, "docx", "rtf", "")
	if err != nil {
		t.Fatalf("docx→rtf: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil || len(body) == 0 {
		t.Fatalf("saved.rtf missing or empty: %v", err)
	}
	rtf := string(body)
	if !strings.Contains(rtf, "\\b") {
		t.Fatalf("rtf missing bold control word: %q", truncate(body, 300))
	}
	if !strings.Contains(convert.RTFPlainText(body), "Demonstration") {
		t.Fatalf("rtf missing document text: %q", truncate(body, 300))
	}
}

func TestSaveChangesTxtUsesDirectPath(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.txt") {
		t.Skip("sample txt missing")
	}

	work := testutil.NewWorkspace(t)
	txtPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.txt")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, txtPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}

	changesDir := filepath.Join(cacheDir, "changes")
	err = os.MkdirAll(changesDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["txt-change"]`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(cacheDir, "saved.txt")
	_, err = conv.SaveChanges(ctx, cacheDir, outPath, "txt")
	if err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "changes-applied.docx")); err == nil {
		t.Fatal("txt save must not use docx bridge")
	}
	if body, err := os.ReadFile(outPath); err != nil || len(body) == 0 {
		t.Fatalf("saved.txt missing or empty: %v", err)
	}
}

func TestToEditorBinRTFSkipsDocxOpenBridge(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.rtf") {
		t.Skip("sample rtf missing")
	}

	work := testutil.NewWorkspace(t)
	rtfPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.rtf")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, rtfPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := os.ReadFile(filepath.Join(cacheDir, "source.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(hash), ":open-docx-v1") {
		t.Fatalf("rtf Editor.bin should use native rtf import, not docx bridge: %q", hash)
	}
	st, err := os.Stat(filepath.Join(cacheDir, "Editor.bin"))
	if err != nil || st.Size() == 0 {
		t.Fatalf("Editor.bin missing or empty: %v", err)
	}
	t.Logf("rtf Editor.bin bytes=%d", st.Size())
}

func TestToEditorBinTxtSkipsDocxOpenBridge(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.txt") {
		t.Skip("sample txt missing")
	}

	work := testutil.NewWorkspace(t)
	txtPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.txt")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, txtPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := os.ReadFile(filepath.Join(cacheDir, "source.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(hash), ":open-docx-v1") {
		t.Fatalf("txt Editor.bin should use native txt import, not docx bridge: %q", hash)
	}
}

func TestSaveChangesCSVAppliesCapturedCellEdit(t *testing.T) {
	t.Skip("coauthoring blobs are session-specific; captured Playwright blobs cannot be replayed on a fresh Editor.bin (see TestSaveChangesCSVConcurrentWithDocumentOpens)")
}

func TestSaveChangesCSVRemapsExcelSheetID(t *testing.T) {
	t.Skip("coauthoring blobs are session-specific; captured Playwright blobs cannot be replayed on a fresh Editor.bin (see TestSaveChangesCSVConcurrentWithDocumentOpens)")
}

func TestPrepareX2TRunDirIsolatesSharedAllFonts(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "AllFonts.js")); err != nil || st.Size() == 0 {
		t.Skip("AllFonts.js missing")
	}

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}

	dir1, err := convert.ExportPrepareX2TRunDir(conv, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir1) })

	dir2, err := convert.ExportPrepareX2TRunDir(conv, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir2) })

	if dir1 == dir2 {
		t.Fatal("expected distinct run directories")
	}
	runRoot := filepath.Join(assets, "converter", "bin", ".run") + string(os.PathSeparator)
	for _, dir := range []string{dir1, dir2} {
		if !strings.HasPrefix(dir, runRoot) {
			t.Fatalf("run dir should live under converter/bin/.run: %s", dir)
		}
		for _, name := range []string{"AllFonts.js", "font_selection.bin", "DoctRenderer.config", "x2t"} {
			path := filepath.Join(dir, name)
			if st, err := os.Stat(path); err != nil || st.Size() == 0 {
				t.Fatalf("%s missing in %s: %v", name, dir, err)
			}
			if name == "x2t" {
				resolved, err := filepath.EvalSymlinks(path)
				if err != nil {
					t.Fatalf("x2t eval symlinks: %v", err)
				}
				if resolved != path {
					t.Fatalf("x2t must be hard-linked in %s, not a symlink to %s", dir, resolved)
				}
			}
		}
		body, err := os.ReadFile(filepath.Join(dir, "DoctRenderer.config"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "<allfonts>./AllFonts.js</allfonts>") {
			t.Fatalf("run-local DoctRenderer.config missing private allfonts: %s", body)
		}
	}
}

func TestAllFontsPathsExist(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "AllFonts.js")); err != nil || st.Size() == 0 {
		t.Skip("AllFonts.js missing")
	}

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	seed := convert.ExportSeedAllFonts(conv)
	paths := convert.ExportAllFontFilePaths(seed)
	if len(paths) == 0 {
		t.Fatal("rewritten AllFonts.js has no font file paths")
	}
	var missing []string
	for _, p := range paths {
		if _, statErr := os.Stat(p); statErr != nil {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d rewritten font paths do not exist (first: %s)", len(missing), missing[0])
	}

	runDir, err := convert.ExportPrepareX2TRunDir(conv, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(runDir) })
	runBody, err := os.ReadFile(filepath.Join(runDir, "AllFonts.js"))
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := filepath.ToSlash(filepath.Join(assets, "core-fonts"))
	if !strings.Contains(string(runBody), wantPrefix) {
		t.Fatalf("isolated AllFonts.js should use current asset dir %s", wantPrefix)
	}
}

func TestToEditorBinSnapshotsFontArtifacts(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}
	work := testutil.NewWorkspace(t)
	csvPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.csv")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, csvPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(cacheDir, "AllFonts.js")); err != nil || st.Size() == 0 {
		t.Fatalf("expected per-document AllFonts.js snapshot in cache dir: %v", err)
	}
	if st, err := os.Stat(filepath.Join(cacheDir, "font_selection.bin")); err != nil || st.Size() == 0 {
		t.Logf("font_selection.bin not snapshotted (optional): %v", err)
	}
}

func TestSaveChangesCSVPlaywrightLiveBlob(t *testing.T) {
	t.Skip("coauthoring blobs are session-specific; captured Playwright blobs cannot be replayed on a fresh Editor.bin (see TestSaveChangesCSVConcurrentWithDocumentOpens)")
}

func TestSharedAllFontsUnmodifiedDuringConcurrentSave(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	sharedAllFonts := filepath.Join(assets, "converter", "bin", "AllFonts.js")
	before, err := os.Stat(sharedAllFonts)
	if err != nil || before.Size() == 0 {
		t.Skip("AllFonts.js missing")
	}
	beforeMod := before.ModTime()

	TestSaveChangesCSVConcurrentWithDocumentOpens(t)

	after, err := os.Stat(sharedAllFonts)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(beforeMod) {
		t.Fatalf("shared %s modtime changed during concurrent save: before=%s after=%s",
			sharedAllFonts, beforeMod, after.ModTime())
	}
}

func TestSaveChangesCSVConcurrentPlaywrightLoad(t *testing.T) {
	// Regression for Playwright save.spec.ts "csv save round-trip" under 10 workers with
	// OFFICE_CONVERT_LIMIT=6. Without per-run DoctRenderer isolation, reverse x2t hits:
	//   exit status 86: CFontFileLoader.LoadFontFromData ... length of null
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	fixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_changes0.json"))
	if err != nil {
		t.Skipf("csv change fixture missing: %v", err)
	}

	const (
		workers      = 10 // PLAYWRIGHT_WORKERS default in playwright.config.ts
		convertLimit = 6 // Playwright load regression (OFFICE_CONVERT_LIMIT=4 in Docker)
	)

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: convertLimit})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < workers; i++ {
		t.Run(fmt.Sprintf("worker-%02d", i), func(t *testing.T) {
			t.Parallel()

			work := testutil.NewWorkspace(t)
			csvPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.csv")))
			cacheDir := t.TempDir()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			if err := conv.ToEditorBin(ctx, csvPath, cacheDir); err != nil {
				t.Fatalf("ToEditorBin: %v", err)
			}
			changesDir := filepath.Join(cacheDir, "changes")
			if err := os.MkdirAll(changesDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), fixture, 0o644); err != nil {
				t.Fatal(err)
			}

			outPath := filepath.Join(cacheDir, "saved.csv")
			if _, err := conv.SaveChanges(ctx, cacheDir, outPath, "csv"); err != nil {
				t.Fatalf("SaveChanges: %v", err)
			}
			body, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "Customer Id") {
				t.Fatalf("csv missing expected content: %q", truncate(body, 120))
			}
			if st, err := os.Stat(filepath.Join(cacheDir, "changes-applied.xlsx")); err != nil || st.Size() == 0 {
				t.Fatalf("expected xlsx bridge output after apply_changes: %v", err)
			}
		})
	}
}

// TestSaveChangesCSVConcurrent is kept as an alias-style smoke name for scripts/docs.
func TestSaveChangesCSVConcurrent(t *testing.T) {
	TestSaveChangesCSVConcurrentPlaywrightLoad(t)
}

func TestSaveChangesCSVConcurrentWithDocumentOpens(t *testing.T) {
	// Playwright runs save tests while other workers open samples (content.spec.ts).
	// Forward ToEditorBin and reverse SaveChanges must each use isolated x2t run dirs
	// with per-document font snapshots; sharing converter/bin/AllFonts.js races.
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") ||
		!testutil.SampleExists(repo, "sample-files/sample.docx") ||
		!testutil.SampleExists(repo, "sample-files/sample.xlsx") ||
		!testutil.SampleExists(repo, "sample-files/sample.pdf") {
		t.Skip("samples missing")
	}

	fixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_changes0.json"))
	if err != nil {
		t.Skipf("csv change fixture missing: %v", err)
	}

	const (
		savers       = 6
		openers      = 10
		convertLimit = 6
	)

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: convertLimit})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	errCh := make(chan error, savers+openers)

	for i := 0; i < openers; i++ {
		go func(n int) {
			samples := []string{
				"sample-files/sample.pdf",
				"sample-files/sample.docx",
				"sample-files/sample.xlsx",
				"sample-files/sample.csv",
			}
			work := testutil.NewWorkspace(t)
			rel := work.CopySample(samples[n%len(samples)])
			abs := filepath.Join(work.Root, filepath.FromSlash(rel))
			cacheDir := t.TempDir()
			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(rel)), ".")
			if ext == "pdf" {
				// Browser-native formats skip x2t on open (see ws.Opener.openBrowserDocument).
				if mkdirErr := os.MkdirAll(cacheDir, 0o755); mkdirErr != nil {
					errCh <- mkdirErr
					return
				}
				data, readErr := os.ReadFile(abs)
				if readErr != nil {
					errCh <- readErr
					return
				}
				if writeErr := os.WriteFile(filepath.Join(cacheDir, "origin.pdf"), data, 0o644); writeErr != nil {
					errCh <- writeErr
					return
				}
				errCh <- nil
				return
			}
			if convErr := conv.ToEditorBin(ctx, abs, cacheDir); convErr != nil {
				errCh <- fmt.Errorf("open worker %d: %w", n, convErr)
				return
			}
			if st, statErr := os.Stat(filepath.Join(cacheDir, "AllFonts.js")); statErr != nil || st.Size() == 0 {
				errCh <- fmt.Errorf("open worker %d missing cache AllFonts.js", n)
				return
			}
			errCh <- nil
		}(i)
	}

	for i := 0; i < savers; i++ {
		go func(n int) {
			work := testutil.NewWorkspace(t)
			csvPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.csv")))
			cacheDir := t.TempDir()
			if convErr := conv.ToEditorBin(ctx, csvPath, cacheDir); convErr != nil {
				errCh <- fmt.Errorf("save worker %d open: %w", n, convErr)
				return
			}
			changesDir := filepath.Join(cacheDir, "changes")
			if mkdirErr := os.MkdirAll(changesDir, 0o755); mkdirErr != nil {
				errCh <- mkdirErr
				return
			}
			if writeErr := os.WriteFile(filepath.Join(changesDir, "changes0.json"), fixture, 0o644); writeErr != nil {
				errCh <- writeErr
				return
			}
			outPath := filepath.Join(cacheDir, "saved.csv")
			if _, saveErr := conv.SaveChanges(ctx, cacheDir, outPath, "csv"); saveErr != nil {
				errCh <- fmt.Errorf("save worker %d: %w", n, saveErr)
				return
			}
			body, readErr := os.ReadFile(outPath)
			if readErr != nil {
				errCh <- readErr
				return
			}
			if !strings.Contains(string(body), "Customer Id") {
				errCh <- fmt.Errorf("save worker %d missing csv content: %q", n, truncate(body, 120))
				return
			}
			if st, statErr := os.Stat(filepath.Join(cacheDir, "changes-applied.xlsx")); statErr != nil || st.Size() == 0 {
				errCh <- fmt.Errorf("save worker %d missing xlsx bridge output: %v", n, statErr)
				return
			}
			errCh <- nil
		}(i)
	}

	for i := 0; i < savers+openers; i++ {
		if recvErr := <-errCh; recvErr != nil {
			t.Fatal(recvErr)
		}
	}
}

func TestToEditorBinReconvertsWhenSourceChanges(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	work := testutil.NewWorkspace(t)
	csvPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.csv")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = conv.ToEditorBin(ctx, csvPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(cacheDir, "Editor.bin"))
	if err != nil {
		t.Fatal(err)
	}
	err = conv.ToEditorBin(ctx, csvPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	sidecar, err := os.ReadFile(filepath.Join(cacheDir, "source.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sidecar) == 0 {
		t.Fatal("expected source.sha256 after convert")
	}

	edited := append([]byte("CHANGED_HEADER,x\n"), []byte{}...)
	orig, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(csvPath, append(edited, orig...), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = conv.ToEditorBin(ctx, csvPath, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(cacheDir, "Editor.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("Editor.bin must be rebuilt when the CSV bytes change under the same cache key")
	}
}

func TestConcurrentToEditorBinSameDir(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.docx") {
		t.Skip("sample docx missing")
	}

	work := testutil.NewWorkspace(t)
	docxPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.docx")))
	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 6})
	if err != nil {
		t.Fatal(err)
	}

	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	const workers = 12
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- conv.ToEditorBin(ctx, docxPath, cacheDir)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	st, err := os.Stat(filepath.Join(cacheDir, "Editor.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() == 0 {
		t.Fatal("Editor.bin is empty")
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "source.sha256")); err != nil {
		t.Fatalf("source.sha256 missing after concurrent convert: %v", err)
	}
}

func zipEntryContains(path, entry, marker string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return false
	}
	for _, f := range zr.File {
		if f.Name != entry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return false
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return false
		}
		return strings.Contains(string(body), marker)
	}
	return false
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
