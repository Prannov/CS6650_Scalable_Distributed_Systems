package store

import "sync"

// Entry holds a value and a monotonically increasing version number.
// Version starts at 1 on first write and increments on every subsequent write.
type Entry struct {
	Value   string
	Version int64
}

// Store is a thread-safe in-memory key-value store.
type Store struct {
	mu   sync.RWMutex
	data map[string]Entry
}

func New() *Store {
	return &Store{data: make(map[string]Entry)}
}

// Set writes a value under key, auto-incrementing the version.
// Returns the new version number.
func (s *Store) Set(key, value string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := int64(1)
	if existing, ok := s.data[key]; ok {
		v = existing.Version + 1
	}
	s.data[key] = Entry{Value: value, Version: v}
	return v
}

// SetWithVersion writes a value with an explicit version (used during replication).
// Only applied if the incoming version is newer than what we have.
func (s *Store) SetWithVersion(key, value string, version int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.data[key]; ok && existing.Version >= version {
		return false
	}
	s.data[key] = Entry{Value: value, Version: version}
	return true
}

// Get returns the Entry for key and true, or zero Entry and false if not found.
func (s *Store) Get(key string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.data[key]
	return e, ok
}