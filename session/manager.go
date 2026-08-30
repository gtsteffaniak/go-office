package session

import (
	"sync"
	"time"
)

// Document holds server-side state for one open document key.
type Document struct {
	Key       string
	Path      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Manager tracks in-memory document sessions (single-process; no Redis).
type Manager struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

func NewManager() *Manager {
	return &Manager{docs: make(map[string]*Document)}
}

func (m *Manager) Get(key string) (*Document, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.docs[key]
	return d, ok
}

func (m *Manager) Upsert(key, path string) *Document {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	if d, ok := m.docs[key]; ok {
		d.Path = path
		d.UpdatedAt = now
		return d
	}
	d := &Document{Key: key, Path: path, CreatedAt: now, UpdatedAt: now}
	m.docs[key] = d
	return d
}

func (m *Manager) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, key)
}

func (m *Manager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.docs)
}
