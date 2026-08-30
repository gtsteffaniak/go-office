package session_test

import (
	"testing"

	"github.com/quantumx-apps/go-office/internal/session"
)

func TestManagerUpsertAndGet(t *testing.T) {
	m := session.NewManager()
	d1 := m.Upsert("k1", "http://example/docs/a.docx")
	if d1.Key != "k1" || d1.URL != "http://example/docs/a.docx" {
		t.Fatalf("unexpected doc: %+v", d1)
	}
	d2 := m.Upsert("k1", "http://example/docs/b.docx")
	if d2.URL != "http://example/docs/b.docx" {
		t.Fatalf("url not updated: %s", d2.URL)
	}
	got, ok := m.Get("k1")
	if !ok || got.URL != "http://example/docs/b.docx" {
		t.Fatalf("get failed: ok=%v doc=%+v", ok, got)
	}
	m.Delete("k1")
	if m.Len() != 0 {
		t.Fatalf("len = %d", m.Len())
	}
}

func TestManagerUpsertDoc(t *testing.T) {
	m := session.NewManager()
	m.UpsertDoc(session.Document{
		Key:         "k2",
		Path:        "sample-files/foo.docx",
		URL:         "http://localhost/file",
		FileType:    "docx",
		CallbackURL: "http://localhost/callback",
	})
	got, ok := m.Get("k2")
	if !ok || got.Path != "sample-files/foo.docx" || got.FileType != "docx" {
		t.Fatalf("doc = %+v", got)
	}
}
