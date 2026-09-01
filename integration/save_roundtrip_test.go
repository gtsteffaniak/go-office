//go:build linux && integration

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/session"
	"github.com/quantumx-apps/go-office/internal/testutil"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestCSVOpenPersistRoundTrip(t *testing.T) {
	dir := assetDir(t)
	if !testutil.SampleExists(testutil.RepoRoot(t), "sample-files/sample.csv") {
		t.Skip("sample-files/sample.csv not in repo")
	}

	work := testutil.NewWorkspace(t)
	csvRel := work.CopySample("sample-files/sample.csv")
	absCSV := filepath.Join(work.Root, filepath.FromSlash(csvRel))

	srv, err := office.New(work.Storage, office.Options{
		AssetDir:     dir,
		ConvertLimit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.Handler()

	docKey := "csv-roundtrip"
	cacheDir := filepath.Join(dir, "cache", docKey)
	conv, err := convert.New(convert.Options{AssetDir: dir, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := conv.ToEditorBin(ctx, absCSV, cacheDir); err != nil {
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

	before, err := os.ReadFile(absCSV)
	if err != nil {
		t.Fatal(err)
	}
	after := work.ReadSample(csvRel)
	if len(after) == 0 {
		t.Fatal("persisted csv is empty")
	}
	if len(before) > 0 && len(after) == 0 {
		t.Fatal("csv lost content after persist")
	}
}

func TestDocxOpenWhileCSVPersist(t *testing.T) {
	dir := assetDir(t)
	repo := testutil.RepoRoot(t)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") || !testutil.SampleExists(repo, "sample-files/sample.docx") {
		t.Skip("sample csv/docx not in repo")
	}

	work := testutil.NewWorkspace(t)
	csvRel := work.CopySample("sample-files/sample.csv")
	docxRel := work.CopySample("sample-files/sample.docx")
	absCSV := filepath.Join(work.Root, filepath.FromSlash(csvRel))
	absDocx := filepath.Join(work.Root, filepath.FromSlash(docxRel))

	srv, err := office.New(work.Storage, office.Options{
		AssetDir:     dir,
		ConvertLimit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.Handler()

	conv, err := convert.New(convert.Options{AssetDir: dir, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}

	csvKey := "csv-key"
	docxKey := "docx-key"
	csvCache := filepath.Join(dir, "cache", csvKey)
	docxCache := filepath.Join(dir, "cache", docxKey)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := conv.ToEditorBin(ctx, absCSV, csvCache); err != nil {
		t.Fatal(err)
	}
	srv.Sessions().UpsertDoc(session.Document{
		Key: csvKey, Path: csvRel, FileType: "csv",
	})

	fileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, absDocx)
	}))
	defer fileSrv.Close()

	var wg sync.WaitGroup
	var persistErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		persistErr = srv.PersistDocument(ctx, csvKey)
	}()

	wg.Add(1)
	var openErr error
	go func() {
		defer wg.Done()
		openErr = conv.ToEditorBin(ctx, absDocx, docxCache)
	}()

	wg.Wait()
	if persistErr != nil {
		t.Fatalf("csv persist: %v", persistErr)
	}
	if openErr != nil {
		t.Fatalf("docx open while csv persist: %v", openErr)
	}
}
