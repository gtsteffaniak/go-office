package convert

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestAcquireConvertSlotRespectsCancel(t *testing.T) {
	c := &Converter{admission: newConvertAdmission(1)}
	release, err := c.acquireConvertSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = c.acquireConvertSlot(ctx)
	if err == nil {
		t.Fatal("expected context error when slot unavailable")
	}
	release()
}

func TestAcquireConvertSlotQueuesWaiters(t *testing.T) {
	c := &Converter{admission: newConvertAdmission(2)}
	release1, err := c.acquireConvertSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release2, err := c.acquireConvertSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	acquired := make(chan struct{})
	go func() {
		release3, err := c.acquireConvertSlot(context.Background())
		if err != nil {
			t.Errorf("queued waiter: %v", err)
			return
		}
		close(acquired)
		release3()
	}()

	select {
	case <-acquired:
		t.Fatal("third waiter should block until a slot is released")
	case <-time.After(30 * time.Millisecond):
	}

	release1()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("queued waiter was not admitted after slot release")
	}
	release2()
}

func TestAcquireConvertSlotQueueFull(t *testing.T) {
	c := &Converter{admission: newConvertAdmissionWithQueue(1, 2)}
	release, err := c.acquireConvertSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			releaseWaiter, acquireErr := c.acquireConvertSlot(ctx)
			if acquireErr != nil {
				t.Errorf("waiter: %v", acquireErr)
				return
			}
			releaseWaiter()
		}()
	}
	time.Sleep(50 * time.Millisecond)

	_, err = c.acquireConvertSlot(context.Background())
	if !errors.Is(err, ErrConvertQueueFull) {
		t.Fatalf("expected ErrConvertQueueFull, got %v", err)
	}
	release()
	wg.Wait()
}

func TestAcquireConvertSlotWaitTimeout(t *testing.T) {
	c := &Converter{admission: newConvertAdmission(1)}
	release, err := c.acquireConvertSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err = c.acquireConvertSlot(ctx)
	if err == nil {
		t.Fatal("expected timeout waiting for slot")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	release()
}

func TestAcquireConvertSlotPriority(t *testing.T) {
	c := &Converter{admission: newConvertAdmission(1)}
	hold, err := c.acquireConvertSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	loDone := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		release, acquireErr := c.acquireConvertSlotLow(ctx)
		if acquireErr != nil {
			t.Errorf("low waiter: %v", acquireErr)
			return
		}
		close(loDone)
		release()
	}()
	time.Sleep(30 * time.Millisecond)

	hiDone := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		release, acquireErr := c.acquireConvertSlot(ctx)
		if acquireErr != nil {
			t.Errorf("high waiter: %v", acquireErr)
			return
		}
		close(hiDone)
		release()
	}()
	time.Sleep(30 * time.Millisecond)

	hold()

	select {
	case <-hiDone:
	case <-time.After(2 * time.Second):
		t.Fatal("high-priority waiter should be admitted before low-priority")
	}
	select {
	case <-loDone:
	case <-time.After(2 * time.Second):
		t.Fatal("low-priority waiter should complete after high-priority")
	}
}

func TestAdmissionNoSlotLeakOnTimeoutRace(t *testing.T) {
	a := newConvertAdmission(1)
	release, err := a.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		_, acquireErr := a.acquire(ctx)
		cancel()
		if acquireErr == nil {
			t.Fatal("expected timeout without releasing holder")
		}
	}
	if a.available() != 0 {
		t.Fatalf("available slots = %d, want 0 while holder active", a.available())
	}
	release()
	if a.available() != 1 {
		t.Fatalf("available slots = %d after release, want 1", a.available())
	}
}

func TestConverterDrainWaitsForInflight(t *testing.T) {
	c := &Converter{admission: newConvertAdmission(1)}
	release, err := c.acquireConvertSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	drainErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		drainErr <- c.Drain(ctx)
	}()
	time.Sleep(20 * time.Millisecond)
	select {
	case err := <-drainErr:
		t.Fatalf("Drain returned early: %v", err)
	default:
	}
	release()
	if err := <-drainErr; err != nil {
		t.Fatalf("Drain: %v", err)
	}
}
