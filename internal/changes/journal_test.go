package changes_test

import (
	"sync"
	"testing"

	"github.com/quantumx-apps/go-office/internal/changes"
)

func TestAppendConcurrent(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := changes.Append(dir, []string{"blob-" + string(rune('a'+n%26))})
			if err != nil {
				t.Errorf("append: %v", err)
			}
		}(i)
	}
	wg.Wait()
	count, err := changes.Count(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 100 {
		t.Fatalf("count = %d, want 100", count)
	}
}

func TestAcknowledgePreservesNewBlobs(t *testing.T) {
	dir := t.TempDir()
	if _, err := changes.Append(dir, []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	if err := changes.Acknowledge(dir, 2); err != nil {
		t.Fatal(err)
	}
	count, err := changes.Count(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}
