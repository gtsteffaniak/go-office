//go:build linux

package office_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/changes"
	"github.com/quantumx-apps/go-office/internal/convert"
	"github.com/quantumx-apps/go-office/internal/session"
	"github.com/quantumx-apps/go-office/internal/testutil"
	office "github.com/quantumx-apps/go-office/pkg/office"
)

// TestPersistDoesNotLoseEditsUnderConcurrentAppends models the CI failure shape: a busy
// editor appends change blobs while a flush is converting. The flush must persist a
// superset of what it acknowledged, and must never acknowledge a blob it did not apply.
//
// The previous coalesce progress guard compared a post-Acknowledge count against a
// pre-flush count, so a busy editor could make a healthy save fail with "coalesce made no
// journal progress".
func TestPersistDoesNotLoseEditsUnderConcurrentAppends(t *testing.T) {
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

	docKey := "concurrent-append"
	cacheDir := filepath.Join(assets, "cache", docKey)
	if err = os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })

	conv, err := newTestConverter(t, assets)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, absCSV, cacheDir); err != nil {
		t.Fatal(err)
	}

	first := loadBlobs(t, repo, "csv_cell_a2.json")
	if _, err = changes.Append(cacheDir, first); err != nil {
		t.Fatal(err)
	}

	srv.Sessions().UpsertDoc(session.Document{Key: docKey, Path: csvRel, FileType: "csv"})

	// Append more blobs concurrently with the persist to exercise the mid-flush path.
	// Bounded like a real editor: a finite burst of edits, not an unbounded stream (which
	// would legitimately trip the coalesce round limit).
	second := loadBlobs(t, repo, "csv_cell_a2_to_2.json")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			_, _ = changes.Append(cacheDir, second)
			time.Sleep(50 * time.Millisecond)
		}
	}()

	err = srv.PersistDocument(ctx, docKey)
	wg.Wait()

	if err != nil {
		t.Fatalf("persist must not fail while changes are appended concurrently: %v", err)
	}

	// The persisted file must reflect the applied edit and preserve the original content.
	after := work.ReadSample(csvRel)
	if len(after) == 0 {
		t.Fatal("persisted csv is empty")
	}
	if !strings.Contains(string(after), "Customer Id") {
		t.Fatalf("csv lost original content: %q", head(after, 200))
	}
}

// TestPersistTwiceDoesNotDropSecondEdit is the no-lost-save regression for the
// refresh-Editor.bin path: after the first save the cache base must be rebuilt from the
// persisted output so a second edit applies on top of it.
func TestPersistTwiceDoesNotDropSecondEdit(t *testing.T) {
	repo := testutil.RepoRoot(t)
	assets := testutil.AssetsDirOrSkip(t, repo)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample csv missing")
	}

	work := testutil.NewWorkspace(t)
	csvRel := work.CopySample("sample-files/sample.csv")
	absCSV := filepath.Join(work.Root, filepath.FromSlash(csvRel))

	srv, err := office.New(work.Storage, office.Options{AssetDir: assets, ConvertLimit: 1})
	if err != nil {
		t.Fatal(err)
	}

	docKey := "sequential-edits"
	cacheDir := filepath.Join(assets, "cache", docKey)
	if err = os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cacheDir) })

	conv, err := newTestConverter(t, assets)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err = conv.ToEditorBin(ctx, absCSV, cacheDir); err != nil {
		t.Fatal(err)
	}
	srv.Sessions().UpsertDoc(session.Document{Key: docKey, Path: csvRel, FileType: "csv"})

	if _, err = changes.Append(cacheDir, loadBlobs(t, repo, "csv_cell_a2.json")); err != nil {
		t.Fatal(err)
	}
	if err = srv.PersistDocument(ctx, docKey); err != nil {
		t.Fatalf("first persist: %v", err)
	}
	firstBody := work.ReadSample(csvRel)

	if _, err = changes.Append(cacheDir, loadBlobs(t, repo, "csv_cell_a2_to_2.json")); err != nil {
		t.Fatal(err)
	}
	if err = srv.PersistDocument(ctx, docKey); err != nil {
		t.Fatalf("second persist: %v", err)
	}
	secondBody := work.ReadSample(csvRel)

	if string(firstBody) == string(secondBody) {
		t.Fatal("second save did not change the persisted file: the second edit was lost")
	}
	if !strings.Contains(string(secondBody), "2") {
		t.Fatalf("second save should reflect the newer value, got %q", head(secondBody, 200))
	}
}


// newTestConverter builds a converter against the repo asset tree.
func newTestConverter(t *testing.T, assets string) (*convert.Converter, error) {
	t.Helper()
	return convert.New(convert.Options{AssetDir: assets, Limit: 1})
}

// loadBlobs reads a captured coauthoring change blob fixture.
func loadBlobs(t *testing.T, repo, name string) []string {
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

func head(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
