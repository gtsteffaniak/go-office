package ws_test

import (
	"testing"

	"github.com/quantumx-apps/go-office/internal/ws"
)

func TestParseBuild(t *testing.T) {
	got := ws.ParseBuild("9.3.4-hotfix.1")
	if got.Release != "9.3.4-hotfix.1" {
		t.Fatalf("Release = %q", got.Release)
	}
	if got.BuildVersion != "9.3.4" {
		t.Fatalf("BuildVersion = %q, want 9.3.4", got.BuildVersion)
	}
	if got.BuildNumber != 0 {
		t.Fatalf("BuildNumber = %d, want 0", got.BuildNumber)
	}
}
