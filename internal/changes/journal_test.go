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

func TestBeginSnapshotPreservesConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	if _, err := changes.Append(dir, []string{"snap-a", "snap-b"}); err != nil {
		t.Fatal(err)
	}
	snap, err := changes.BeginSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if snap.BlobCount != 2 {
		t.Fatalf("blob count = %d, want 2", snap.BlobCount)
	}
	if _, err := changes.Append(dir, []string{"after-snap"}); err != nil {
		t.Fatal(err)
	}
	count, err := changes.Count(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("live count = %d, want 3 after snapshot", count)
	}
	if err := changes.Acknowledge(dir, snap.BlobCount); err != nil {
		t.Fatal(err)
	}
	count, err = changes.Count(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count after ack = %d, want 1 remaining blob", count)
	}
	changes.RemoveSnapshot(dir)
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
