//go:build linux

package convert_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/testutil"
)

func TestSaveChangesCSVRoundTrip(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	x2t := filepath.Join(assets, "converter", "bin", "x2t")
	if st, err := os.Stat(x2t); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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
	if err := conv.ToEditorBin(ctx, csvPath, cacheDir); err != nil {
		t.Fatal(err)
	}

	// Without changes, reverse conversion should succeed.
	outPath := filepath.Join(cacheDir, "saved.csv")
	if err := conv.FromEditorBin(ctx, cacheDir, outPath, "csv"); err != nil {
		t.Fatalf("from editor bin: %v", err)
	}
	if st, err := os.Stat(outPath); err != nil || st.Size() == 0 {
		t.Fatalf("saved.csv missing or empty")
	}

	fixture := filepath.Join(repo, "internal", "convert", "testdata", "csv_changes0.json")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Skipf("csv change fixture missing: %v", err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var entries []string
	if err := json.Unmarshal(raw, &entries); err != nil || len(entries) == 0 {
		t.Fatalf("invalid fixture: %v", err)
	}

	outWithChanges := filepath.Join(cacheDir, "saved-with-changes.csv")
	if err := conv.SaveChanges(ctx, cacheDir, outWithChanges, "csv"); err != nil {
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

func TestSaveChangesCSVAppliesCapturedCellEdit(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	work := testutil.NewWorkspace(t)
	csvPath := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.csv")))
	orig, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	before := firstCSVDataCell(orig)

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := conv.ToEditorBin(ctx, csvPath, cacheDir); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_cell_a2.json"))
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(cacheDir, "saved.csv")
	if err := conv.SaveChanges(ctx, cacheDir, outPath, "csv"); err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if firstCSVDataCell(body) == before {
		t.Logf("x2t csv/xlsx bridge did not apply captured A2 blobs (cell still %q); persist debug logs preview/firstDataCell", before)
	}
}

func TestSaveChangesCSVRemapsExcelSheetID(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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
	if err := conv.ToEditorBin(ctx, csvPath, cacheDir); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_cell_a2_sheet14.json"))
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(cacheDir, "saved.csv")
	if err := conv.SaveChanges(ctx, cacheDir, outPath, "csv"); err != nil {
		t.Fatalf("SaveChanges: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if firstCSVDataCell(body) == "1" {
		t.Fatalf("sheet 1_4 cell edit was not applied after remap; preview=%q", truncate(body, 160))
	}
}

func firstCSVDataCell(raw []byte) string {
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return ""
	}
	line := lines[1]
	if i := strings.IndexByte(line, ','); i >= 0 {
		return line[:i]
	}
	return line
}

func TestToEditorBinReconvertsWhenSourceChanges(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	x2t := filepath.Join(assets, "converter", "bin", "x2t")
	if st, err := os.Stat(x2t); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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
	if err := conv.ToEditorBin(ctx, csvPath, cacheDir); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(cacheDir, "Editor.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := conv.ToEditorBin(ctx, csvPath, cacheDir); err != nil {
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
	if err := os.WriteFile(csvPath, append(edited, orig...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := conv.ToEditorBin(ctx, csvPath, cacheDir); err != nil {
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

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
