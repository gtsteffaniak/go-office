package demo

import (
	"context"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-push/push"
)

const warmDebounceInterval = 400 * time.Millisecond

type warmJob struct {
	ctx context.Context
	doc sampleDoc
}

type warmScheduler struct {
	mu     sync.Mutex
	pacers map[string]*push.Pacer[warmJob]
	h      *Handler
}

func newWarmScheduler(h *Handler) *warmScheduler {
	return &warmScheduler{
		pacers: make(map[string]*push.Pacer[warmJob]),
		h:      h,
	}
}

func (s *warmScheduler) schedule(ctx context.Context, doc sampleDoc) {
	s.mu.Lock()
	p, ok := s.pacers[doc.Key]
	if !ok {
		p = push.New[warmJob](push.Config{
			Mode:      push.ModeDebounce,
			Interval:  warmDebounceInterval,
			QueueSize: 8,
		})
		s.pacers[doc.Key] = p
		go s.consume(p)
	}
	s.mu.Unlock()
	p.Push(warmJob{ctx: ctx, doc: doc})
}

func (s *warmScheduler) consume(p *push.Pacer[warmJob]) {
	for job := range p.Updates() {
		warmErr := s.h.warmDocument(job.ctx, job.doc)
		if warmErr == nil || s.h.opts.Logger == nil {
			continue
		}
		if s.h.office.EditorBinCached(job.doc.Key) {
			s.h.opts.Logger.Debug("demo warm lost race; editor cache ready",
				"file", job.doc.RelPath, "key", job.doc.Key, "err", warmErr)
			continue
		}
		s.h.opts.Logger.Error("demo warm failed", "file", job.doc.RelPath, "key", job.doc.Key, "err", warmErr)
	}
}
