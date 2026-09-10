package convert

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultQueueWaitTimeout bounds how long callers wait for a convert slot when
// their context has no deadline. Direct API users may pass a shorter deadline.
const DefaultQueueWaitTimeout = 2 * time.Minute

// convertQueueMaxWaiters is the maximum number of blocked x2t admission waiters.
// Beyond this, new acquire calls fail immediately with ErrConvertQueueFull.
const convertQueueMaxWaiters = 500

// ErrConvertQueueFull is returned when more than convertQueueMaxWaiters callers
// are already waiting for a convert slot.
var ErrConvertQueueFull = errors.New("convert: admission queue full")

type slotGrant struct {
	handoff chan func()
	done    chan struct{}
}

// convertAdmission limits concurrent x2t subprocesses with prioritized FIFO wait queues.
// High-priority waiters (editor open, save) are admitted before low-priority work
// (demo thumbnails). go-push is used for demo warm debounce; admission here uses
// explicit channels because go-push Push() drops after 50ms when its buffer is full.
type convertAdmission struct {
	limit       int
	maxWait     int
	queueWait   time.Duration
	slots       chan struct{}
	hiIncoming  chan slotGrant
	loIncoming  chan slotGrant
	waiting     atomic.Int32
	inflight    sync.WaitGroup
}

func newConvertAdmission(limit int) *convertAdmission {
	return newConvertAdmissionWithQueue(limit, convertQueueMaxWaiters, DefaultQueueWaitTimeout)
}

func newConvertAdmissionWithQueue(limit, maxWaiters int, queueWait time.Duration) *convertAdmission {
	if limit <= 0 {
		limit = 1
	}
	if maxWaiters <= 0 {
		maxWaiters = convertQueueMaxWaiters
	}
	if queueWait <= 0 {
		queueWait = DefaultQueueWaitTimeout
	}
	a := &convertAdmission{
		limit:      limit,
		maxWait:    maxWaiters,
		queueWait:  queueWait,
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
		grant := a.waitForGrant()
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
		release := func() {
			a.slots <- struct{}{}
			a.inflight.Done()
		}
		select {
		case grant.handoff <- release:
		case <-grant.done:
			release()
		}
	}
}

// waitForGrant blocks until a waiter is queued, preferring hi over lo.
func (a *convertAdmission) waitForGrant() slotGrant {
	select {
	case grant := <-a.hiIncoming:
		return grant
	default:
	}
	for {
		select {
		case grant := <-a.hiIncoming:
			return grant
		case grant := <-a.loIncoming:
			select {
			case hi := <-a.hiIncoming:
				a.requeueLo(grant)
				return hi
			default:
				return grant
			}
		}
	}
}

func (a *convertAdmission) requeueLo(grant slotGrant) {
	select {
	case a.loIncoming <- grant:
	default:
		go func(g slotGrant) { a.loIncoming <- g }(grant)
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

func withQueueWait(ctx context.Context, queueWait time.Duration) (context.Context, context.CancelFunc) {
	if queueWait <= 0 {
		queueWait = DefaultQueueWaitTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.Now().Add(queueWait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		return ctx, func() {}
	}
	return context.WithDeadline(ctx, deadline)
}

func (a *convertAdmission) acquireOn(ctx context.Context, incoming chan slotGrant) (func(), error) {
	ctx, cancelWait := withQueueWait(ctx, a.queueWait)
	defer cancelWait()

	if int(a.waiting.Add(1)) > a.maxWait {
		a.waiting.Add(-1)
		return nil, ErrConvertQueueFull
	}

	a.inflight.Add(1)
	grant := slotGrant{
		handoff: make(chan func()),
		done:    make(chan struct{}),
	}

	select {
	case incoming <- grant:
	case <-ctx.Done():
		a.waiting.Add(-1)
		a.inflight.Done()
		return nil, ctx.Err()
	}

	select {
	case release := <-grant.handoff:
		a.waiting.Add(-1)
		return release, nil
	case <-ctx.Done():
		close(grant.done)
		select {
		case release := <-grant.handoff:
			a.waiting.Add(-1)
			release()
		default:
			a.waiting.Add(-1)
			a.inflight.Done()
		}
		return nil, ctx.Err()
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
