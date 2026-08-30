package callback_test

import (
	"strings"
	"testing"

	"github.com/quantumx-apps/go-office/pkg/callback"
)

func TestParseAndShouldPersist(t *testing.T) {
	p, err := callback.Parse([]byte(`{"key":"abc","status":2,"url":"http://localhost/saved.docx"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "abc" || p.Status != callback.StatusMustSave || !p.ShouldPersist() {
		t.Fatalf("payload = %+v", p)
	}

	p, err = callback.Parse([]byte(`{"key":"abc","status":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.ShouldPersist() {
		t.Fatal("editing status should not persist")
	}

	p, err = callback.Parse([]byte(`{"status":6}`))
	if err == nil || !strings.Contains(err.Error(), "key") {
		t.Fatalf("expected key error, got %v", err)
	}
}
