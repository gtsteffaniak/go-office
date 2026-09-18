package envconfig

import "testing"

func TestJWTSecretFallback(t *testing.T) {
	t.Setenv("OFFICE_JWT_SECRET", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_ENABLED", "")
	t.Setenv("OFFICE_JWT_ENABLED", "")
	if got := JWTSecret(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	t.Setenv("JWT_SECRET", "onlyoffice-secret")
	if got := JWTSecret(); got != "onlyoffice-secret" {
		t.Fatalf("JWT_SECRET fallback = %q", got)
	}

	t.Setenv("OFFICE_JWT_SECRET", "office-secret")
	if got := JWTSecret(); got != "office-secret" {
		t.Fatalf("OFFICE_JWT_SECRET must win, got %q", got)
	}
}

func TestJWTSecretDisabled(t *testing.T) {
	t.Setenv("OFFICE_JWT_SECRET", "")
	t.Setenv("JWT_SECRET", "onlyoffice-secret")
	t.Setenv("JWT_ENABLED", "false")
	if got := JWTSecret(); got != "" {
		t.Fatalf("JWT_ENABLED=false must disable secret, got %q", got)
	}

	t.Setenv("JWT_ENABLED", "")
	t.Setenv("OFFICE_JWT_ENABLED", "false")
	if got := JWTSecret(); got != "" {
		t.Fatalf("OFFICE_JWT_ENABLED=false must disable secret, got %q", got)
	}
}

func TestFirstAndBool(t *testing.T) {
	t.Setenv("A", "")
	t.Setenv("B", "value")
	if got := First("A", "B", "C"); got != "value" {
		t.Fatalf("First = %q", got)
	}
	t.Setenv("X", "true")
	if !Bool("X") {
		t.Fatal("expected true")
	}
}
