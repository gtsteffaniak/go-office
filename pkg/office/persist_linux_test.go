//go:build linux

package office_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/changes"
	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/session"
	"github.com/quantumx-apps/go-office/internal/testutil"
	"github.com/quantumx-apps/go-office/internal/ws"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestPersistCSVAppliesPendingChanges(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	work := testutil.NewWorkspace(t)
	csvRel := work.CopySample("sample-files/sample.csv")
	absCSV := filepath.Join(work.Root, filepath.FromSlash(csvRel))

	srv, err := office.New(work.Storage, office.Options{
		AssetDir:     assets,
		ConvertLimit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	docKey := "csv-persist-changes"
	cacheDir := filepath.Join(assets, "cache", docKey)
	if err = os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, absCSV, cacheDir); err != nil {
		t.Fatal(err)
	}

	fixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_changes0.json"))
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changesDir, "changes0.json"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}

	srv.Sessions().UpsertDoc(session.Document{
		Key:      docKey,
		Path:     csvRel,
		FileType: "csv",
	})
	if err := srv.PersistDocument(ctx, docKey); err != nil {
		t.Fatal(err)
	}

	after := work.ReadSample(csvRel)
	if len(after) == 0 {
		t.Fatal("persisted csv is empty")
	}
	if !strings.Contains(string(after), "Customer Id") {
		t.Fatalf("csv lost original content after persist: %q", after[:min(200, len(after))])
	}
	var blobs []string
	if err := json.Unmarshal(fixture, &blobs); err != nil || len(blobs) == 0 {
		t.Fatal("fixture empty")
	}
}

func TestPersistCSVThenDocxDoesNotBlock(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") || !testutil.SampleExists(repo, "sample-files/sample.docx") {
		t.Skip("samples missing")
	}

	work := testutil.NewWorkspace(t)
	csvRel := work.CopySample("sample-files/sample.csv")
	docxRel := work.CopySample("sample-files/sample.docx")
	absCSV := filepath.Join(work.Root, filepath.FromSlash(csvRel))
	absDocx := filepath.Join(work.Root, filepath.FromSlash(docxRel))

	srv, err := office.New(work.Storage, office.Options{
		AssetDir:     assets,
		ConvertLimit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	csvKey := "switch-csv"
	docxKey := "switch-docx"
	csvCache := filepath.Join(assets, "cache", csvKey)
	docxCache := filepath.Join(assets, "cache", docxKey)
	t.Cleanup(func() {
		_ = os.RemoveAll(csvCache)
		_ = os.RemoveAll(docxCache)
	})

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := conv.ToEditorBin(ctx, absCSV, csvCache); err != nil {
		t.Fatal(err)
	}

	srv.Sessions().UpsertDoc(session.Document{Key: csvKey, Path: csvRel, FileType: "csv"})
	if err := srv.PersistDocument(ctx, csvKey); err != nil {
		t.Fatal(err)
	}

	if err := conv.ToEditorBin(ctx, absDocx, docxCache); err != nil {
		t.Fatalf("docx open after csv persist: %v", err)
	}
	if st, err := os.Stat(filepath.Join(docxCache, "Editor.bin")); err != nil || st.Size() == 0 {
		t.Fatal("docx Editor.bin missing after switch")
	}
	_ = docxRel
}

func TestPersistInvalidatesEditorBin(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.txt") {
		t.Skip("sample txt missing")
	}

	work := testutil.NewWorkspace(t)
	txtRel := work.CopySample("sample-files/sample.txt")
	absTxt := filepath.Join(work.Root, filepath.FromSlash(txtRel))

	srv, err := office.New(work.Storage, office.Options{
		AssetDir:     assets,
		ConvertLimit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	docKey := "persist-invalidate-txt"
	cacheDir := filepath.Join(assets, "cache", docKey)
	if err = os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, absTxt, cacheDir); err != nil {
		t.Fatal(err)
	}

	changesDir := filepath.Join(cacheDir, "changes")
	if err = os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), []byte(`["txt-change"]`), 0o644); err != nil {
		t.Fatal(err)
	}

	srv.Sessions().UpsertDoc(session.Document{
		Key:      docKey,
		Path:     txtRel,
		FileType: "txt",
	})
	if err = srv.PersistDocument(ctx, docKey); err != nil {
		t.Fatal(err)
	}
	// No active coauthoring session in this unit test — cache is invalidated for reopen.
	if _, err := os.Stat(filepath.Join(cacheDir, "Editor.bin")); !os.IsNotExist(err) {
		t.Fatal("Editor.bin should be removed after persist so reopen reconverts from storage")
	}
	if _, err := os.Stat(changesDir); !os.IsNotExist(err) {
		t.Fatal("changes dir should be cleared after persist")
	}
}

func TestPersistRefreshesEditorBinBeforeAck(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	work := testutil.NewWorkspace(t)
	csvRel := work.CopySample("sample-files/sample.csv")
	absCSV := filepath.Join(work.Root, filepath.FromSlash(csvRel))

	srv, err := office.New(work.Storage, office.Options{AssetDir: assets, ConvertLimit: 2})
	if err != nil {
		t.Fatal(err)
	}

	docKey := "persist-refresh-editor-bin"
	cacheDir := filepath.Join(assets, "cache", docKey)
	if err = os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, absCSV, cacheDir); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_cell_a2.json"))
	if err != nil {
		t.Fatal(err)
	}
	changesDir := filepath.Join(cacheDir, "changes")
	if err = os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(changesDir, "changes0.json"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}

	hashBefore, err := os.ReadFile(filepath.Join(cacheDir, "source.sha256"))
	if err != nil {
		t.Fatal(err)
	}

	ws.SetActiveDocumentSessionForTest(docKey)
	srv.Sessions().UpsertDoc(session.Document{Key: docKey, Path: csvRel, FileType: "csv"})
	if err = srv.FlushDocument(ctx, docKey, "http://localhost", true); err != nil {
		t.Fatal(err)
	}
	persisted := work.ReadSample(csvRel)
	if !bytes.Contains(persisted, []byte("2")) {
		t.Fatalf("fixture changes should alter saved storage bytes, got %q", persisted[:min(120, len(persisted))])
	}
	hashAfter, err := os.ReadFile(filepath.Join(cacheDir, "source.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	if string(hashBefore) == string(hashAfter) {
		t.Fatal("flush should refresh Editor.bin from saved output and update source.sha256")
	}
	st, err := os.Stat(filepath.Join(cacheDir, "Editor.bin"))
	if err != nil || st.Size() == 0 {
		t.Fatal("active session should keep refreshed Editor.bin after flush")
	}
}

func TestPersistCoalesceMidFlushSteps(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	work := testutil.NewWorkspace(t)
	csvRel := work.CopySample("sample-files/sample.csv")
	absCSV := filepath.Join(work.Root, filepath.FromSlash(csvRel))

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, absCSV, cacheDir); err != nil {
		t.Fatal(err)
	}

	firstFixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_cell_a2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var firstBlobs []string
	if err = json.Unmarshal(firstFixture, &firstBlobs); err != nil {
		t.Fatal(err)
	}
	if _, err = changes.Append(cacheDir, firstBlobs); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(cacheDir, "saved.csv")
	ackFirst, err := conv.SaveChanges(ctx, cacheDir, outPath, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if ackFirst == 0 {
		t.Fatal("expected first convert to acknowledge blobs")
	}

	secondFixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_cell_a2_to_2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var secondBlobs []string
	if err = json.Unmarshal(secondFixture, &secondBlobs); err != nil {
		t.Fatal(err)
	}
	if _, err = changes.Append(cacheDir, secondBlobs); err != nil {
		t.Fatal(err)
	}

	pending, err := changes.Count(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if pending <= ackFirst {
		t.Fatalf("expected extra blobs after mid-flush append, pending=%d ackFirst=%d", pending, ackFirst)
	}

	if err = conv.ToEditorBin(ctx, outPath, cacheDir); err != nil {
		t.Fatal(err)
	}
	if err = changes.Acknowledge(cacheDir, ackFirst); err != nil {
		t.Fatal(err)
	}
	ackSecond, err := conv.SaveChanges(ctx, cacheDir, outPath, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if ackSecond == 0 {
		t.Fatal("expected second convert to acknowledge remaining blobs")
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("2")) {
		t.Fatalf("coalesced saves should reflect both batches, got %q", body[:min(120, len(body))])
	}
}

func TestSequentialSaveChangesWithEditorBinRefresh(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	work := testutil.NewWorkspace(t)
	csvRel := work.CopySample("sample-files/sample.csv")
	absCSV := filepath.Join(work.Root, filepath.FromSlash(csvRel))

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, absCSV, cacheDir); err != nil {
		t.Fatal(err)
	}

	firstFixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_cell_a2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var firstBlobs []string
	if err = json.Unmarshal(firstFixture, &firstBlobs); err != nil {
		t.Fatal(err)
	}
	if _, err = changes.Append(cacheDir, firstBlobs); err != nil {
		t.Fatal(err)
	}
	saved1 := filepath.Join(cacheDir, "saved1.csv")
	if _, err = conv.SaveChanges(ctx, cacheDir, saved1, "csv"); err != nil {
		t.Fatal(err)
	}
	if err = conv.ToEditorBin(ctx, saved1, cacheDir); err != nil {
		t.Fatal(err)
	}

	secondFixture, err := os.ReadFile(filepath.Join(repo, "internal", "convert", "testdata", "csv_cell_a2_to_2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var secondBlobs []string
	if err = json.Unmarshal(secondFixture, &secondBlobs); err != nil {
		t.Fatal(err)
	}
	if _, err = changes.Append(cacheDir, secondBlobs); err != nil {
		t.Fatal(err)
	}
	saved2 := filepath.Join(cacheDir, "saved2.csv")
	if _, err = conv.SaveChanges(ctx, cacheDir, saved2, "csv"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(saved2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("2")) {
		t.Fatalf("second save should reflect refreshed base, got %q", body[:min(120, len(body))])
	}
}
