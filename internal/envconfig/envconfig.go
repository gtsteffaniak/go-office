// Package envconfig resolves go-office settings from environment variables.
// OFFICE_* names are preferred; ONLYOFFICE Document Server names are accepted
// as fallbacks where noted in migration.md.
package envconfig

import (
	"os"
	"strings"
)

// First returns the first non-empty trimmed value among keys.
func First(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// Bool reports true when any key is set to 1/true/yes (case-insensitive).
func Bool(keys ...string) bool {
	for _, key := range keys {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes":
			return true
		}
	}
	return false
}

// ExplicitBool reports whether any key was set to a boolean-like value and the
// parsed result. Unset keys return (false, false).
func ExplicitBool(keys ...string) (value, set bool) {
	for _, key := range keys {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes":
			return true, true
		case "0", "false", "no":
			return false, true
		}
	}
	return false, false
}

// JWTEnabled mirrors ONLYOFFICE JWT_ENABLED (default: enabled when a secret is set).
func JWTEnabled() bool {
	if enabled, set := ExplicitBool("OFFICE_JWT_ENABLED", "JWT_ENABLED"); set {
		return enabled
	}
	return true
}

// JWTSecret prefers OFFICE_JWT_SECRET, then ONLYOFFICE JWT_SECRET.
// Returns empty when JWT is explicitly disabled via OFFICE_JWT_ENABLED / JWT_ENABLED=false.
func JWTSecret() string {
	if !JWTEnabled() {
		return ""
	}
	return First("OFFICE_JWT_SECRET", "JWT_SECRET")
}
