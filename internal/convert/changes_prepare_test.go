package convert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareChangesForSaveRewritesJSON(t *testing.T) {
	dir := t.TempDir()
	blobs := []string{"14;CgAAAAFiAAAA/wAAAAA=", "37;AgAAAAM="}
	raw, err := json.Marshal(blobs)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "changes0.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepareChangesForSave(dir, ""); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `["14;CgAAAAFiAAAA/wAAAAA=","37;AgAAAAM="]`
	if string(got) != want {
		t.Fatalf("changes json = %q want %q", got, want)
	}
}

func TestApplyChangesFontPaths(t *testing.T) {
	runDir := "/tmp/x2t-run"
	fontDir, allFonts := applyChangesFontPaths(runDir)
	if fontDir != runDir {
		t.Fatalf("fontDir = %q", fontDir)
	}
	if allFonts != "/tmp/x2t-run/AllFonts.js" {
		t.Fatalf("allFonts = %q", allFonts)
	}
}
