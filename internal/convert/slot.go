package convert

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// convertQueueMaxWaiters is the maximum number of blocked x2t admission waiters.
// Beyond this, new acquire calls fail immediately with ErrConvertQueueFull.
const convertQueueMaxWaiters = 500

// convertQueueWaitTimeout is how long a caller may wait for a convert slot before
// the request expires (avoids indefinite buildup under sustained load).
const convertQueueWaitTimeout = 10 * time.Second

// ErrConvertQueueFull is returned when more than convertQueueMaxWaiters callers
// are already waiting for a convert slot.
var ErrConvertQueueFull = errors.New("convert: admission queue full")

type slotGrant struct {
	ready chan struct{}
	done  chan struct{}
}

// convertAdmission limits concurrent x2t subprocesses with a FIFO wait queue.
// Waiters block until a slot is available, their context is cancelled, or
// convertQueueWaitTimeout elapses. go-push is used for demo warm debounce;
// admission here uses an explicit FIFO channel because go-push Push() is
// non-blocking and drops after 50ms when its input buffer is full.
type convertAdmission struct {
	limit     int
	maxWait   int
	slots     chan struct{}
	incoming  chan slotGrant
	waiting   atomic.Int32
	inflight  sync.WaitGroup
}

func newConvertAdmission(limit int) *convertAdmission {
	return newConvertAdmissionWithQueue(limit, convertQueueMaxWaiters)
}

func newConvertAdmissionWithQueue(limit, maxWaiters int) *convertAdmission {
	if limit <= 0 {
		limit = 1
	}
	if maxWaiters <= 0 {
		maxWaiters = convertQueueMaxWaiters
	}
	a := &convertAdmission{
		limit:    limit,
		maxWait:  maxWaiters,
		slots:    make(chan struct{}, limit),
		incoming: make(chan slotGrant, 64),
	}
	for i := 0; i < limit; i++ {
		a.slots <- struct{}{}
	}
	go a.run()
	return a
}

func (a *convertAdmission) run() {
	for grant := range a.incoming {
		if grantIsCancelled(grant) {
			continue
		}
		select {
		case <-a.slots:
		case <-grant.done:
			continue
		}
		if grantIsCancelled(grant) {
			a.slots <- struct{}{}
			continue
		}
		close(grant.ready)
	}
}

func grantIsCancelled(grant slotGrant) bool {
	select {
	case <-grant.done:
		return true
	default:
		return false
	}
}

func (a *convertAdmission) acquire(ctx context.Context) (func(), error) {
	if int(a.waiting.Add(1)) > a.maxWait {
		a.waiting.Add(-1)
		return nil, ErrConvertQueueFull
	}

	a.inflight.Add(1)
	grant := slotGrant{
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}

	waitCtx, cancel := context.WithTimeout(ctx, convertQueueWaitTimeout)
	defer cancel()

	select {
	case a.incoming <- grant:
	case <-waitCtx.Done():
		a.waiting.Add(-1)
		a.inflight.Done()
		return nil, waitCtx.Err()
	}

	select {
	case <-grant.ready:
		a.waiting.Add(-1)
		return func() {
			a.slots <- struct{}{}
			a.inflight.Done()
		}, nil
	case <-waitCtx.Done():
		close(grant.done)
		a.waiting.Add(-1)
		a.inflight.Done()
		return nil, waitCtx.Err()
	}
}

func (a *convertAdmission) drain(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		a.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *convertAdmission) stop() {
	close(a.incoming)
}

// Drain waits until in-flight conversions finish or ctx is cancelled.
func (c *Converter) Drain(ctx context.Context) error {
	if c == nil || c.admission == nil {
		return nil
	}
	return c.admission.drain(ctx)
}
