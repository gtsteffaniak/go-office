package ws

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	docchanges "github.com/quantumx-apps/go-office/internal/changes"
)

type midFlushAckSaver struct {
	cacheDir string
	ackBlobs int

	started chan struct{}
	resume  chan struct{}
}

func (s *midFlushAckSaver) FlushDocument(_ context.Context, docKey, _ string, _ bool) error {
	if s.started != nil {
		s.started <- struct{}{}
		<-s.resume
	}
	cache := filepath.Join(s.cacheDir, docKey)
	return docchanges.Acknowledge(cache, s.ackBlobs)
}

func TestRunOneFlushKeepsChangesAppendedDuringFlush(t *testing.T) {
	cacheDir := t.TempDir()
	docKey := "key"
	docCache := filepath.Join(cacheDir, docKey)

	for i := 0; i < 7; i++ {
		if _, err := appendChanges(docCache, []string{"blob"}); err != nil {
			t.Fatal(err)
		}
	}

	saver := &midFlushAckSaver{
		cacheDir: cacheDir,
		ackBlobs: 7,
		started:  make(chan struct{}, 1),
		resume:   make(chan struct{}),
	}
	sched := newSaveScheduler(cacheDir, saver, nil, nil, nil)

	var flushDone sync.WaitGroup
	flushDone.Add(1)
	go func() {
		defer flushDone.Done()
		if err := sched.runOneFlush(docKey, "", true); err != nil {
			t.Errorf("runOneFlush: %v", err)
		}
	}()

	select {
	case <-saver.started:
	case <-time.After(time.Second):
		t.Fatal("flush did not start")
	}

	for i := 0; i < 5; i++ {
		if _, err := appendChanges(docCache, []string{"late"}); err != nil {
			t.Fatal(err)
		}
	}
	close(saver.resume)

	flushDone.Wait()

	count, err := docchanges.Count(docCache)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("expected 5 blobs remaining after partial ack, got %d", count)
	}
}
