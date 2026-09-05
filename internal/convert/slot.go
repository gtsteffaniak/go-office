package convert

import (
	"context"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-push/push"
)

const (
	convertAdmissionInterval = time.Millisecond
	convertAdmissionQueue    = 10_000
)

type slotGrant struct {
	ready chan struct{}
	done  chan struct{}
}

// convertAdmission limits concurrent x2t subprocesses with a go-push rate-limited
// admission queue. Excess callers wait in order until a slot frees; they are not
// rejected while their context remains valid.
type convertAdmission struct {
	limit     int
	slots     chan struct{}
	admission *push.Pacer[slotGrant]
	inflight  sync.WaitGroup
}

func newConvertAdmission(limit int) *convertAdmission {
	if limit <= 0 {
		limit = 1
	}
	a := &convertAdmission{
		limit: limit,
		slots: make(chan struct{}, limit),
		admission: push.New[slotGrant](push.Config{
			Mode:      push.ModeRateLimit,
			Interval:  convertAdmissionInterval,
			MaxItems:  limit,
			QueueSize: convertAdmissionQueue,
		}),
	}
	for i := 0; i < limit; i++ {
		a.slots <- struct{}{}
	}
	go a.run()
	return a
}

func (a *convertAdmission) run() {
	for grant := range a.admission.Updates() {
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
	a.inflight.Add(1)
	grant := slotGrant{
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
	a.admission.Push(grant)
	select {
	case <-grant.ready:
		return func() {
			a.slots <- struct{}{}
			a.inflight.Done()
		}, nil
	case <-ctx.Done():
		close(grant.done)
		a.inflight.Done()
		return nil, ctx.Err()
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
	if a.admission != nil {
		a.admission.Stop()
	}
}

// Drain waits until in-flight conversions finish or ctx is cancelled.
func (c *Converter) Drain(ctx context.Context) error {
	if c == nil || c.admission == nil {
		return nil
	}
	return c.admission.drain(ctx)
}
