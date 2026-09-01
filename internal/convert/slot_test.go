package convert

import (
	"context"
	"testing"
	"time"
)

func TestAcquireConvertSlotRespectsCancel(t *testing.T) {
	c := &Converter{limit: make(chan struct{}, 1)}
	c.limit <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.acquireConvertSlot(ctx)
	if err == nil {
		t.Fatal("expected context error when slot unavailable")
	}
}

func TestConverterDrainWaitsForInflight(t *testing.T) {
	c := &Converter{limit: make(chan struct{}, 1)}
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
