package ws

import (
	"sync"
	"time"
)

// SaveOutcome describes the authoritative result of a document flush. It is broadcast to
// every coauthoring session for the document key so clients (and the demo viewer) can
// report save status without polling storage.
//
// Why this exists: the sdkjs client clears its 60s save-retry timer on any unSaveLock
// reply, which the server sends as soon as changes are journaled — before the debounced
// x2t flush runs. A failed or stalled flush is therefore invisible to the editor, and the
// only observer used to be a client-side disk poll. This event closes that gap.
type SaveOutcome struct {
	// Type is always "saveResult".
	Type string `json:"type"`
	// Key is the document key the outcome applies to.
	Key string `json:"key"`
	// Success reports whether the flush persisted the document.
	Success bool `json:"success"`
	// Force mirrors the flush's force flag (true for manual/force saves).
	Force bool `json:"force"`
	// RolledBack reports that the target format could not be written and OOXML bridge bytes
	// were persisted at the original path (assemblyFormatAsOrigin behaviour).
	RolledBack bool `json:"rolledBack"`
	// Bridge names the OOXML format persisted when RolledBack is true.
	Bridge string `json:"bridge,omitempty"`
	// Bytes is the size of the persisted file on success.
	Bytes int64 `json:"bytes,omitempty"`
	// Sequence is a per-key monotonically increasing counter so clients can drop stale
	// events that arrive out of order.
	Sequence uint64 `json:"sequence"`
	// Time is the server time in unix milliseconds when the flush settled.
	Time int64 `json:"time"`
	// Error carries the failure reason when Success is false.
	Error string `json:"error,omitempty"`
}

// broadcastSaveOutcome records the outcome for later queries and sends a save-result
// event to all sessions of docKey.
func broadcastSaveOutcome(key string, outcome SaveOutcome) {
	if key == "" {
		return
	}
	if outcome.Type == "" {
		outcome.Type = "saveResult"
	}
	if outcome.Time == 0 {
		outcome.Time = time.Now().UnixMilli()
	}
	recordSaveOutcome(key, outcome)
	pkt, err := socketMessage(map[string]any{
		"type":       outcome.Type,
		"key":        outcome.Key,
		"success":    outcome.Success,
		"force":      outcome.Force,
		"rolledBack": outcome.RolledBack,
		"bridge":     outcome.Bridge,
		"bytes":      outcome.Bytes,
		"sequence":   outcome.Sequence,
		"time":       outcome.Time,
		"error":      outcome.Error,
	})
	if err != nil {
		return
	}
	forEachSession(key, func(s *session) {
		s.enqueue(pkt)
	})
}

// saveOutcomeLedger retains the most recent save outcome per document key so HTTP clients
// (the demo viewer) can read authoritative save state.
var saveOutcomeLedger = struct {
	mu       sync.RWMutex
	outcomes map[string]SaveOutcome
}{outcomes: make(map[string]SaveOutcome)}

func recordSaveOutcome(key string, outcome SaveOutcome) {
	saveOutcomeLedger.mu.Lock()
	saveOutcomeLedger.outcomes[key] = outcome
	saveOutcomeLedger.mu.Unlock()
}

// LastSaveOutcome returns the most recent recorded save outcome for key.
func LastSaveOutcome(key string) (SaveOutcome, bool) {
	saveOutcomeLedger.mu.RLock()
	defer saveOutcomeLedger.mu.RUnlock()
	out, ok := saveOutcomeLedger.outcomes[key]
	return out, ok
}

// ResetSaveOutcomesForTest clears the retained outcomes (tests only).
func ResetSaveOutcomesForTest() {
	saveOutcomeLedger.mu.Lock()
	saveOutcomeLedger.outcomes = make(map[string]SaveOutcome)
	saveOutcomeLedger.mu.Unlock()
}
