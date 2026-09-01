package changes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quantumx-apps/go-office/internal/changes"
)

func TestHasPending(t *testing.T) {
	dir := t.TempDir()
	if changes.HasPending(dir) {
		t.Fatal("empty cache should have no pending changes")
	}
	chDir := filepath.Join(dir, "changes")
	if err := os.MkdirAll(chDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chDir, changes.FileName), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changes.HasPending(dir) {
		t.Fatal("empty changes0.json should not count as pending")
	}
	if err := os.WriteFile(filepath.Join(chDir, changes.FileName), []byte(`["blob"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !changes.HasPending(dir) {
		t.Fatal("non-empty changes0.json should be pending")
	}
}
