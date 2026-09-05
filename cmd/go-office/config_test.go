package main

import (
	"testing"
	"time"

	"github.com/quantumx-apps/go-office/internal/envconfig"
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
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_ENABLED", "")
	if got := envconfig.JWTSecret(); got != "test-secret" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("OFFICE_JWT_SECRET", "")
	if got := envconfig.JWTSecret(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	t.Setenv("JWT_SECRET", "onlyoffice-secret")
	if got := envconfig.JWTSecret(); got != "onlyoffice-secret" {
		t.Fatalf("JWT_SECRET fallback = %q", got)
	}
	t.Setenv("JWT_ENABLED", "false")
	if got := envconfig.JWTSecret(); got != "" {
		t.Fatalf("JWT_ENABLED=false must clear secret, got %q", got)
	}
}
