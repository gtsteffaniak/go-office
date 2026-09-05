package ws

import (
	"time"

	"github.com/gtsteffaniak/go-cache/cache"
)

// documentConfigEpochs tracks editor config generations per document key.
// Bumped when integrators reset coauthoring (BuildEditorConfig, session reset).
// Sessions compare against their configEpochAtCreate to detect stale state without
// relying solely on in-memory session deletion (belt-and-suspenders for reload).
var documentConfigEpochs = cache.NewCache[uint64](30 * time.Minute)

func bumpDocumentConfigEpoch(docKey string) {
	if docKey == "" {
		return
	}
	v, ok := documentConfigEpochs.Get(docKey)
	next := uint64(1)
	if ok {
		next = v + 1
	}
	documentConfigEpochs.Set(docKey, next)
}

func documentConfigEpoch(docKey string) uint64 {
	if docKey == "" {
		return 0
	}
	v, ok := documentConfigEpochs.Get(docKey)
	if !ok {
		return 0
	}
	return v
}

// resetDocumentConfigEpochs clears epoch cache (tests only).
func resetDocumentConfigEpochs() {
	documentConfigEpochs.ClearAll()
}
