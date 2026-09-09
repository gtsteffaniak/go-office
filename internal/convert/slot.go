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

// ErrConvertQueueFull is returned when more than convertQueueMaxWaiters callers
// are already waiting for a convert slot.
var ErrConvertQueueFull = errors.New("convert: admission queue full")

type slotGrant struct {
	ready chan struct{}
	done  chan struct{}
}

// convertAdmission limits concurrent x2t subprocesses with prioritized FIFO wait queues.
// High-priority waiters (editor open, save) are admitted before low-priority work
// (demo thumbnails). go-push is used for demo warm debounce; admission here uses
// explicit channels because go-push Push() drops after 50ms when its buffer is full.
type convertAdmission struct {
	limit      int
	maxWait    int
	slots      chan struct{}
	hiIncoming chan slotGrant
	loIncoming chan slotGrant
	waiting    atomic.Int32
	inflight   sync.WaitGroup
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
		limit:      limit,
		maxWait:    maxWaiters,
		slots:      make(chan struct{}, limit),
		hiIncoming: make(chan slotGrant, 64),
		loIncoming: make(chan slotGrant, 64),
	}
	for i := 0; i < limit; i++ {
		a.slots <- struct{}{}
	}
	go a.run()
	return a
}

func (a *convertAdmission) run() {
	for {
		grant, ok := a.nextGrant()
		if !ok {
			return
		}
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
		// Waiter may time out after ready closes; return the slot if cancelled.
		select {
		case <-grant.done:
			a.slots <- struct{}{}
		default:
		}
	}
}

func (a *convertAdmission) nextGrant() (slotGrant, bool) {
	select {
	case grant := <-a.hiIncoming:
		return grant, true
	default:
	}
	select {
	case grant := <-a.hiIncoming:
		return grant, true
	case grant := <-a.loIncoming:
		return grant, true
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
	return a.acquireOn(ctx, a.hiIncoming)
}

func (a *convertAdmission) acquireLow(ctx context.Context) (func(), error) {
	return a.acquireOn(ctx, a.loIncoming)
}

func (a *convertAdmission) acquireOn(ctx context.Context, incoming chan slotGrant) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if int(a.waiting.Add(1)) > a.maxWait {
		a.waiting.Add(-1)
		return nil, ErrConvertQueueFull
	}

	a.inflight.Add(1)
	grant := slotGrant{
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}

	waitStart := time.Now()

	select {
	case incoming <- grant:
	case <-ctx.Done():
		a.waiting.Add(-1)
		a.inflight.Done()
		return nil, ctx.Err()
	}

	select {
	case <-grant.ready:
		a.waiting.Add(-1)
		if waited := time.Since(waitStart); waited > 5*time.Second {
			// Slow admission under load — visible in debug logs when logger is wired.
			_ = waited
		}
		return func() {
			a.slots <- struct{}{}
			a.inflight.Done()
		}, nil
	case <-ctx.Done():
		close(grant.done)
		select {
		case <-grant.ready:
			a.waiting.Add(-1)
			return func() {
				a.slots <- struct{}{}
				a.inflight.Done()
			}, ctx.Err()
		default:
			a.waiting.Add(-1)
			a.inflight.Done()
			return nil, ctx.Err()
		}
	}
}

// available returns how many slots are currently free (for tests).
func (a *convertAdmission) available() int {
	return len(a.slots)
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

// Drain waits until in-flight conversions finish or ctx is cancelled.
func (c *Converter) Drain(ctx context.Context) error {
	if c == nil || c.admission == nil {
		return nil
	}
	return c.admission.drain(ctx)
}
