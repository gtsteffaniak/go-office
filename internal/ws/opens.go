package ws

import (
	"context"
	"sync"
)

var docOpenInflight sync.WaitGroup

func waitDocumentOpens(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		docOpenInflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
