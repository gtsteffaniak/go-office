package office_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	office "github.com/quantumx-apps/go-office/pkg/office"
)

func TestCacheJanitorEvictsOldDirs(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "oldkey")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(old, oldTime, oldTime)

	j := office.NewCacheJanitorForTest(dir, time.Hour, 10, nil)
	j.SweepOnce()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("expected old cache dir removed")
	}
}

func TestServerCloseStops(t *testing.T) {
	srv, err := office.New(nopStorage{}, office.Options{AssetDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	srv.Handler() // register coauthoring
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	_ = context.Background()
}
