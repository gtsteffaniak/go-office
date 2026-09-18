package session

import (
	"sync"
	"time"
)

// Document holds server-side state for one open document key.
type Document struct {
	Key         string
	Path        string // host storage path (VFS-relative)
	URL         string // document download URL used on open
	FileType    string // extension without dot, e.g. docx
	CallbackURL string
	UserID      string // editor user id from coauthoring auth
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Manager tracks in-memory document sessions (single-process; no Redis).
type Manager struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

func NewManager() *Manager {
	return &Manager{docs: make(map[string]*Document)}
}

// Lookup returns a defensive copy of document metadata for key.
func (m *Manager) Lookup(key string) (Document, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.docs[key]
	if !ok {
		return Document{}, false
	}
	return *d, true
}

// Get is an alias for Lookup.
func (m *Manager) Get(key string) (Document, bool) {
	return m.Lookup(key)
}

// Upsert records document URL for a key (legacy helper).
func (m *Manager) Upsert(key, docURL string) Document {
	return m.UpsertDoc(Document{Key: key, URL: docURL})
}

// UpsertDoc stores or updates document metadata.
func (m *Manager) UpsertDoc(doc Document) Document {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	if d, ok := m.docs[doc.Key]; ok {
		if doc.Path != "" {
			d.Path = doc.Path
		}
		if doc.URL != "" {
			d.URL = doc.URL
		}
		if doc.FileType != "" {
			d.FileType = doc.FileType
		}
		if doc.CallbackURL != "" {
			d.CallbackURL = doc.CallbackURL
		}
		if doc.UserID != "" {
			d.UserID = doc.UserID
		}
		d.UpdatedAt = now
		return *d
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	stored := doc
	m.docs[doc.Key] = &stored
	return stored
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
