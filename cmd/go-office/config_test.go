package main

import (
	"testing"
	"time"
)

func TestPollHoldFromEnv(t *testing.T) {
	t.Setenv("OFFICE_POLL_HOLD", "0")
	if got := pollHoldFromEnv(); got == nil || *got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
	t.Setenv("OFFICE_POLL_HOLD", "500ms")
	if got := pollHoldFromEnv(); got == nil || *got != 500*time.Millisecond {
		t.Fatalf("expected 500ms, got %v", got)
	}
	t.Setenv("OFFICE_POLL_HOLD", "")
	if got := pollHoldFromEnv(); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestConvertLimitFromEnv(t *testing.T) {
	t.Setenv("OFFICE_CONVERT_LIMIT", "2")
	if got := convertLimitFromEnv(); got != 2 {
		t.Fatalf("got %d", got)
	}
	t.Setenv("OFFICE_CONVERT_LIMIT", "")
	if got := convertLimitFromEnv(); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestDebugFromEnv(t *testing.T) {
	t.Setenv("OFFICE_DEBUG_LOGGING", "1")
	t.Setenv("OFFICE_DEBUG", "")
	if !debugFromEnv() {
		t.Fatal("expected debug enabled")
	}
	t.Setenv("OFFICE_DEBUG_LOGGING", "")
	if debugFromEnv() {
		t.Fatal("expected debug disabled")
	}
}

func TestJWTSecretFromEnv(t *testing.T) {
	t.Setenv("OFFICE_JWT_SECRET", "test-secret")
	if got := jwtSecretFromEnv(); got != "test-secret" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("OFFICE_JWT_SECRET", "")
	if got := jwtSecretFromEnv(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	// ONLYOFFICE JWT_SECRET is not read — use OFFICE_JWT_SECRET (see migration.md).
	t.Setenv("JWT_SECRET", "legacy-onlyoffice-name")
	if got := jwtSecretFromEnv(); got != "" {
		t.Fatalf("JWT_SECRET must not be used, got %q", got)
	}
}
