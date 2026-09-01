package office_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quantumx-apps/go-office/internal/changes"
)

func TestHasPendingChangesMatchesChangesPackage(t *testing.T) {
	dir := t.TempDir()
	chDir := filepath.Join(dir, "changes")
	if err := os.MkdirAll(chDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if changes.HasPending(dir) {
		t.Fatal("expected no pending changes")
	}
	if err := os.WriteFile(filepath.Join(chDir, changes.FileName), []byte(`["x"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !changes.HasPending(dir) {
		t.Fatal("expected pending changes")
	}
}
