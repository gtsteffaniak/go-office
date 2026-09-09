package session_test

import (
	"sync"
	"testing"

	"github.com/quantumx-apps/go-office/internal/session"
)

func TestManagerConcurrentUpsertLookup(t *testing.T) {
	m := session.NewManager()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key"
			m.UpsertDoc(session.Document{
				Key:      key,
				Path:     "path-" + string(rune('a'+n%26)),
				FileType: "docx",
			})
			doc, ok := m.Lookup(key)
			if !ok || doc.Key != key {
				t.Errorf("lookup failed for %s", key)
			}
		}(i)
	}
	wg.Wait()
	if m.Len() != 1 {
		t.Fatalf("len = %d, want 1", m.Len())
	}
}
