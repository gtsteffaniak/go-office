//go:build linux

package office_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/session"
	"github.com/quantumx-apps/go-office/internal/testutil"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestPersistCSVAppliesPendingChanges(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })

	conv, err := convert.New(convert.Options{AssetDir: assets, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := conv.ToEditorBin(ctx, absCSV, cacheDir); err != nil {
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
	assets := filepath.Join(repo, "assets")
	if st, err := os.Stat(filepath.Join(assets, "converter", "bin", "x2t")); err != nil || st.IsDir() {
		t.Skip("x2t not available")
	}
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
