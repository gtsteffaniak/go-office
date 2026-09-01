package debuglog

import "testing"

func TestEnvEnabled(t *testing.T) {
	t.Setenv("OFFICE_DEBUG_LOGGING", "")
	t.Setenv("OFFICE_DEBUG", "")
	if EnvEnabled() {
		t.Fatal("expected false")
	}
	t.Setenv("OFFICE_DEBUG_LOGGING", "1")
	if !EnvEnabled() {
		t.Fatal("expected true for OFFICE_DEBUG_LOGGING")
	}
	t.Setenv("OFFICE_DEBUG_LOGGING", "")
	t.Setenv("OFFICE_DEBUG", "true")
	if !EnvEnabled() {
		t.Fatal("expected true for legacy OFFICE_DEBUG")
	}
}
