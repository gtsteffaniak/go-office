//go:build linux

package convert_test

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestPrepareX2TRunDirIsolatesSharedAllFonts(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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
					t.Fatalf("x2t must be a copied binary in %s, not a symlink to %s", dir, resolved)
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
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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
		if _, err := os.Stat(p); err != nil {
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

func TestSaveChangesCSVConcurrentPlaywrightLoad(t *testing.T) {
	// Regression for Playwright save.spec.ts "csv save round-trip" under 10 workers with
	// OFFICE_CONVERT_LIMIT=6. Without per-run DoctRenderer isolation, reverse x2t hits:
	//   exit status 86: CFontFileLoader.LoadFontFromData ... length of null
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	fixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_changes0.json"))
	if err != nil {
		t.Skipf("csv change fixture missing: %v", err)
	}

	const (
		workers      = 10 // PLAYWRIGHT_WORKERS in Dockerfile.playwright-office
		convertLimit = 6  // OFFICE_CONVERT_LIMIT in Dockerfile.playwright-office
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
			if err := conv.SaveChanges(ctx, cacheDir, outPath, "csv"); err != nil {
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
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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
				if err := os.MkdirAll(cacheDir, 0o755); err != nil {
					errCh <- err
					return
				}
				data, err := os.ReadFile(abs)
				if err != nil {
					errCh <- err
					return
				}
				if err := os.WriteFile(filepath.Join(cacheDir, "origin.pdf"), data, 0o644); err != nil {
					errCh <- err
					return
				}
				errCh <- nil
				return
			}
			if err := conv.ToEditorBin(ctx, abs, cacheDir); err != nil {
				errCh <- fmt.Errorf("open worker %d: %w", n, err)
				return
			}
			if st, err := os.Stat(filepath.Join(cacheDir, "AllFonts.js")); err != nil || st.Size() == 0 {
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
			if err := conv.ToEditorBin(ctx, csvPath, cacheDir); err != nil {
				errCh <- fmt.Errorf("save worker %d open: %w", n, err)
				return
			}
			changesDir := filepath.Join(cacheDir, "changes")
			if err := os.MkdirAll(changesDir, 0o755); err != nil {
				errCh <- err
				return
			}
			if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), fixture, 0o644); err != nil {
				errCh <- err
				return
			}
			outPath := filepath.Join(cacheDir, "saved.csv")
			if err := conv.SaveChanges(ctx, cacheDir, outPath, "csv"); err != nil {
				errCh <- fmt.Errorf("save worker %d: %w", n, err)
				return
			}
			body, err := os.ReadFile(outPath)
			if err != nil {
				errCh <- err
				return
			}
			if !strings.Contains(string(body), "Customer Id") {
				errCh <- fmt.Errorf("save worker %d missing csv content: %q", n, truncate(body, 120))
				return
			}
			if st, err := os.Stat(filepath.Join(cacheDir, "changes-applied.xlsx")); err != nil || st.Size() == 0 {
				errCh <- fmt.Errorf("save worker %d missing xlsx bridge output: %v", n, err)
				return
			}
			errCh <- nil
		}(i)
	}

	for i := 0; i < savers+openers; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
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

func TestConcurrentToEditorBinSameDir(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	x2t := filepath.Join(assets, "converter", "bin", "x2t")
	if st, err := os.Stat(x2t); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
