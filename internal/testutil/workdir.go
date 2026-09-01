// Package testutil provides isolated workspaces for tests that mutate documents.
package testutil

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

const samplesDirName = "sample-files"

// Workspace is an isolated data root: copied samples, cache dir, and disk-backed Storage.
type Workspace struct {
	T          *testing.T
	Root       string
	SamplesDir string
	CacheDir   string
	Storage    office.Storage
	repoRoot   string
}

// NewWorkspace creates a per-test directory tree under t.TempDir().
func NewWorkspace(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	repo := RepoRoot(t)
	w := &Workspace{
		T:          t,
		Root:       root,
		SamplesDir: samplesDirName,
		CacheDir:   filepath.Join(root, "cache"),
		repoRoot:   repo,
	}
	w.Storage = &diskStorage{root: root}
	if err := os.MkdirAll(w.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return w
}

// RepoRoot walks up from the working directory to find the module root (go.mod).
func RepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// CopySample copies a tracked sample from the repo into the workspace and returns its storage path.
func (w *Workspace) CopySample(repoRel string) string {
	w.T.Helper()
	repoRel = filepath.ToSlash(repoRel)
	src := filepath.Join(w.repoRoot, filepath.FromSlash(repoRel))
	if _, err := os.Stat(src); err != nil {
		w.T.Skipf("sample not found: %s (%v)", repoRel, err)
	}
	dst := filepath.Join(w.Root, filepath.FromSlash(repoRel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		w.T.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		w.T.Fatal(err)
	}
	return repoRel
}

// ReadSample reads a file from the workspace by storage-relative path.
func (w *Workspace) ReadSample(storagePath string) []byte {
	w.T.Helper()
	full := filepath.Join(w.Root, filepath.FromSlash(storagePath))
	data, err := os.ReadFile(full)
	if err != nil {
		w.T.Fatal(err)
	}
	return data
}

// SampleExists reports whether a repo sample is present (for matrix checks).
func SampleExists(repoRoot, repoRel string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(repoRel)))
	return err == nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

type diskStorage struct {
	root string
}

func (s *diskStorage) Open(_ context.Context, path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.root, filepath.FromSlash(path)))
}

func (s *diskStorage) Save(_ context.Context, path string, r io.Reader) error {
	full := filepath.Join(s.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *diskStorage) Stat(_ context.Context, path string) (office.FileInfo, error) {
	full := filepath.Join(s.root, filepath.FromSlash(path))
	fi, err := os.Stat(full)
	if err != nil {
		return office.FileInfo{}, err
	}
	return office.FileInfo{
		Path:    strings.ReplaceAll(path, "\\", "/"),
		Name:    fi.Name(),
		Size:    fi.Size(),
		ModTime: fi.ModTime().UTC().Truncate(time.Second),
	}, nil
}
