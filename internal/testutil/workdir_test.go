package testutil_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/internal/testutil"
)

func TestWorkspaceCopySampleIsolated(t *testing.T) {
	repo := testutil.RepoRoot(t)
	if !testutil.SampleExists(repo, "sample-files/sample.csv") {
		t.Skip("sample-files/sample.csv not in repo")
	}

	work := testutil.NewWorkspace(t)
	rel := work.CopySample("sample-files/sample.csv")
	src := filepath.Join(repo, filepath.FromSlash(rel))
	dst := filepath.Join(work.Root, filepath.FromSlash(rel))

	srcData, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dstData := work.ReadSample(rel)
	if string(srcData) != string(dstData) {
		t.Fatal("copied sample should match repo fixture")
	}

	marker := []byte("PLAYWRIGHT_SHOULD_NOT_TOUCH_REPO")
	if err = os.WriteFile(dst, marker, 0o644); err != nil {
		t.Fatal(err)
	}
	repoData, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(repoData), "PLAYWRIGHT_SHOULD_NOT_TOUCH_REPO") {
		t.Fatal("workspace mutation leaked to tracked sample-files/")
	}
}
