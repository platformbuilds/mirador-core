package session

import (
	"sync"
	"time"
)

type SessionData struct {
	Scope            string
	LastHealth       map[string]interface{}
	LastFailures     []map[string]interface{}
	LastRCA          map[string]interface{}
	PendingMutations []map[string]interface{}
	UpdatedAt        time.Time
}

type Store struct {
	mu    sync.RWMutex
	ttl   time.Duration
	store map[string]SessionData
}

func NewStore(ttl time.Duration) *Store {
	return &Store{
		ttl:   ttl,
		store: make(map[string]SessionData),
	}
}

func (s *Store) Get(id string) (SessionData, bool) {
	s.mu.RLock()
	sd, ok := s.store[id]
	s.mu.RUnlock()
	if !ok {
		return SessionData{}, false
	}
	if time.Since(sd.UpdatedAt) > s.ttl {
		return SessionData{}, false
	}
	return sd, true
}

func (s *Store) Set(id string, sd *SessionData) {
	if sd == nil {
		return
	}
	sd.UpdatedAt = time.Now()
	s.mu.Lock()
	s.store[id] = *sd
	s.mu.Unlock()
}

func (s *Store) Reset(id string) {
	s.mu.Lock()
	delete(s.store, id)
	s.mu.Unlock()
}

// SessionStore is the interface for session storage, implemented by Store.
type SessionStore interface {
	Get(id string) (SessionData, bool)
	Set(id string, sd *SessionData)
	Reset(id string)
	Ensure(id string) SessionData
}

// Ensure returns an existing session or creates a default one if not present.
func (s *Store) Ensure(id string) SessionData {
	s.mu.RLock()
	sd, ok := s.store[id]
	s.mu.RUnlock()
	if ok && time.Since(sd.UpdatedAt) <= s.ttl {
		return sd
	}
	s.mu.Lock()
	// Double-check after acquiring write lock
	sd, ok = s.store[id]
	if !ok {
		sd = SessionData{Scope: "default", UpdatedAt: time.Now()}
		s.store[id] = sd
	}
	s.mu.Unlock()
	return sd
}
