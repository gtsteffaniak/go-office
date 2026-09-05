package convert

import (
	"context"
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
