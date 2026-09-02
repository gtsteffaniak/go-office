package testutil

import (
	"os"
	"path/filepath"
	"testing"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

// AssetsDirOrSkip returns the Euro-Office assets directory for integration tests.
// When OFFICE_ASSETS is set (CI), missing x2t is a hard failure instead of a skip.
func AssetsDirOrSkip(t *testing.T, repo string) string {
	t.Helper()
	assets := office.AssetDirFromEnv(filepath.Join(repo, "assets"))
	x2t := filepath.Join(assets, "converter", "bin", "x2t")
	if st, err := os.Stat(x2t); err != nil || st.IsDir() {
		if os.Getenv("OFFICE_ASSETS") != "" {
			t.Fatalf("x2t not available at %s but OFFICE_ASSETS is set", x2t)
		}
		t.Skip("x2t not available")
	}
	return assets
}
