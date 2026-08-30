package session_test

import (
	"testing"

	"github.com/quantumx-apps/go-office/internal/session"
)

func TestManagerUpsertAndGet(t *testing.T) {
	m := session.NewManager()
	d1 := m.Upsert("k1", "/docs/a.docx")
	if d1.Key != "k1" || d1.Path != "/docs/a.docx" {
		t.Fatalf("unexpected doc: %+v", d1)
	}
	d2 := m.Upsert("k1", "/docs/b.docx")
	if d2.Path != "/docs/b.docx" {
		t.Fatalf("path not updated: %s", d2.Path)
	}
	got, ok := m.Get("k1")
	if !ok || got.Path != "/docs/b.docx" {
		t.Fatalf("get failed: ok=%v doc=%+v", ok, got)
	}
	m.Delete("k1")
	if m.Len() != 0 {
		t.Fatalf("len = %d", m.Len())
	}
}
