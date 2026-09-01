package convert

import "context"

func (c *Converter) acquireConvertSlot(ctx context.Context) (func(), error) {
	c.inflight.Add(1)
	select {
	case c.limit <- struct{}{}:
		return func() { <-c.limit; c.inflight.Done() }, nil
	case <-ctx.Done():
		c.inflight.Done()
		return nil, ctx.Err()
	}
}

// Drain waits until in-flight conversions finish or ctx is cancelled.
func (c *Converter) Drain(ctx context.Context) error {
	if c == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		c.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
