package main

import (
	"testing"
)

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
