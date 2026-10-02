//go:build linux

package convert_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/changes"
	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/testutil"
)

// writeChanges appends blobs to cacheDir/changes/changes0.json.
func writeChanges(t *testing.T, cacheDir string, blobs []string) {
	t.Helper()
	if _, err := changes.Append(cacheDir, blobs); err != nil {
		t.Fatalf("append changes: %v", err)
	}
}

// loadChangeFixture reads a captured coauthoring change blob fixture.
func loadChangeFixture(t *testing.T, repo, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var blobs []string
	if err := json.Unmarshal(raw, &blobs); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	if len(blobs) == 0 {
		t.Fatalf("fixture %s is empty", name)
	}
	return blobs
}

// TestSaveChangesODSWithPendingChanges is the regression test for the Playwright
// post-save-stability ODS failure.
//
// The pre-existing TestSaveChangesODSRoundTrip calls SaveChanges with an EMPTY journal, so it
// only exercises the direct fromEditorInner path and never the xlsx→ods bridge that a real
// edit takes. That gap is why CI caught this failure and the unit suite did not.
//
// ODS must genuinely convert (x2t can write OpenDocument) and must NOT silently fall back to
// xlsx bytes: a rolled-back ODS would corrupt the document for ODF consumers.
func TestSaveChangesODSWithPendingChanges(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if convErr := conv.ToEditorBin(ctx, src, cacheDir); convErr != nil {
		t.Fatalf("ToEditorBin: %v", convErr)
	}

	blobs := loadChangeFixture(t, repo, "csv_cell_a2.json")
	writeChanges(t, cacheDir, blobs)

	outPath := filepath.Join(cacheDir, "saved.ods")
	res, err := conv.SaveChangesDetailed(ctx, cacheDir, outPath, "ods")
	if err != nil {
		t.Fatalf("ODS SaveChanges with pending changes: %v", err)
	}
	if res.AckBlobs == 0 {
		t.Fatal("expected converted blobs to be acknowledged")
	}
	if res.RolledBack {
		t.Fatal("ods must not roll back to OOXML: x2t can write OpenDocument")
	}
	if res.Bytes == 0 {
		t.Fatal("saved.ods is empty")
	}

	// A real ODS is an ODF package with a content.xml entry.
	if !zipEntryContains(outPath, "content.xml", "office:document-content") {
		t.Fatal("saved.ods is not a valid OpenDocument package (missing content.xml)")
	}
	// It must not be an OOXML workbook masquerading as ODS.
	if zipEntryContains(outPath, "xl/workbook.xml", "") {
		t.Fatal("saved.ods contains OOXML parts: the xlsx bridge leaked into an ODS save")
	}
}

// TestSaveChangesXlsRollsBackToOOXML pins the assemblyFormatAsOrigin contract for .xls.
//
// Verified empirically against the bundled x2t: xlsx→xls exits with status 88, so a real
// binary Excel file cannot be produced. Upstream responds by persisting the OOXML bytes at
// the original path ("rollback to save changes to ooxml"), and go-office mirrors that.
// This test asserts the rollback is deliberate and REPORTED, not silent.
func TestSaveChangesXlsRollsBackToOOXML(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.xlsx") {
		t.Skip("sample xlsx missing")
	}

	work := testutil.NewWorkspace(t)
	src := filepath.Join(work.Root, filepath.FromSlash(work.CopySample("sample-files/sample.xlsx")))

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if convErr := conv.ToEditorBin(ctx, src, cacheDir); convErr != nil {
		t.Fatalf("ToEditorBin: %v", convErr)
	}
	writeChanges(t, cacheDir, loadChangeFixture(t, repo, "csv_cell_a2.json"))

	outPath := filepath.Join(cacheDir, "saved.xls")
	res, err := conv.SaveChangesDetailed(ctx, cacheDir, outPath, "xls")
	if err != nil {
		t.Fatalf("XLS SaveChanges: %v", err)
	}
	if res.AckBlobs == 0 {
		t.Fatal("expected converted blobs to be acknowledged")
	}
	if !res.RolledBack {
		t.Fatal("xls save should report a rollback (x2t cannot write binary Excel: exit 88)")
	}
	if res.Bridge != "xlsx" {
		t.Fatalf("rollback bridge = %q, want xlsx", res.Bridge)
	}
	if res.Bytes == 0 {
		t.Fatal("saved.xls is empty")
	}
	// The persisted bytes are the OOXML bridge output.
	if !zipEntryContains(outPath, "xl/workbook.xml", "") {
		t.Fatal("rolled-back .xls should contain the OOXML bridge workbook")
	}
	intermediate := filepath.Join(cacheDir, "changes-applied.xlsx")
	want, err := os.ReadFile(intermediate)
	if err != nil {
		t.Fatalf("read bridge output: %v", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read saved.xls: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("rolled-back .xls should be byte-identical to changes-applied.xlsx")
	}
}

// TestSaveChangesODSDoesNotRollBackOnSuccess guards the asymmetry that caused the CI
// failure: previously only xls/doc/ppt had a fallback branch, and ods returned (0, err)
// with no bytes written, stranding the journal so no later save could make progress.
func TestSaveChangesFailureLeavesJournalIntact(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if convErr := conv.ToEditorBin(ctx, src, cacheDir); convErr != nil {
		t.Fatalf("ToEditorBin: %v", convErr)
	}

	writeChanges(t, cacheDir, loadChangeFixture(t, repo, "csv_cell_a2.json"))
	before, err := changes.Count(cacheDir)
	if err != nil {
		t.Fatal(err)
	}

	// Force a conversion failure with a cancelled context; the journal must survive so the
	// save can be retried rather than losing the user's edits.
	deadCtx, cancelDead := context.WithCancel(ctx)
	cancelDead()
	if _, deadErr := conv.SaveChangesDetailed(deadCtx, cacheDir, filepath.Join(cacheDir, "saved.ods"), "ods"); deadErr == nil {
		t.Fatal("expected SaveChanges to fail with a cancelled context")
	}
	after, err := changes.Count(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if after < before {
		t.Fatalf("failed save must not acknowledge blobs: before=%d after=%d", before, after)
	}
}

// TestSaveResultReportsSnapshotIdentity verifies the conversion reports which journal
// entries it consumed, so acknowledgement can be verified instead of assumed.
func TestSaveResultReportsSnapshotIdentity(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if convErr := conv.ToEditorBin(ctx, src, cacheDir); convErr != nil {
		t.Fatalf("ToEditorBin: %v", convErr)
	}
	blobs := loadChangeFixture(t, repo, "csv_cell_a2.json")
	writeChanges(t, cacheDir, blobs)

	res, err := conv.SaveChangesDetailed(ctx, cacheDir, filepath.Join(cacheDir, "saved.ods"), "ods")
	if err != nil {
		t.Fatalf("SaveChangesDetailed: %v", err)
	}
	if res.Snapshot.BlobCount != len(blobs) {
		t.Fatalf("snapshot blob count = %d, want %d", res.Snapshot.BlobCount, len(blobs))
	}
	if len(res.Snapshot.Blobs) != len(blobs) {
		t.Fatalf("snapshot blobs = %d, want %d", len(res.Snapshot.Blobs), len(blobs))
	}
	// The snapshot must be acknowledged transactionally against the journal it came from.
	removed, err := changes.AcknowledgeSnapshot(cacheDir, res.Snapshot)
	if err != nil {
		t.Fatalf("AcknowledgeSnapshot: %v", err)
	}
	if removed != len(blobs) {
		t.Fatalf("acknowledged %d blobs, want %d", removed, len(blobs))
	}
	if changes.HasPending(cacheDir) {
		t.Fatal("journal should be empty after acknowledging the converted snapshot")
	}
}

// TestAcknowledgeSnapshotRejectsDivergentJournal ensures a stale snapshot cannot delete
// edits it never converted.
func TestAcknowledgeSnapshotRejectsDivergentJournal(t *testing.T) {
	cacheDir := t.TempDir()
	writeChanges(t, cacheDir, []string{"first"})
	snap, err := changes.BeginSnapshot(cacheDir)
	if err != nil {
		t.Fatalf("BeginSnapshot: %v", err)
	}
	// Replace the journal contents so the snapshot is no longer a prefix.
	if clearErr := changes.Clear(cacheDir); clearErr != nil {
		t.Fatal(clearErr)
	}
	writeChanges(t, cacheDir, []string{"different"})

	removed, err := changes.AcknowledgeSnapshot(cacheDir, snap)
	if err == nil {
		t.Fatal("expected AcknowledgeSnapshot to reject a divergent journal")
	}
	if removed != 0 {
		t.Fatalf("divergent acknowledgement removed %d blobs, want 0", removed)
	}
	remaining, err := changes.Count(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("journal should be untouched, have %d blobs", remaining)
	}
}
