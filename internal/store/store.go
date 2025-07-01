// Package store contains the core logic for the in-memory key-value store.
// It is designed to be thread-safe for concurrent access.
package store

import "sync"

// VersionedValue holds the actual value and a version number for concurrency control.
type VersionedValue struct {
	Value   string
	Version uint64
}

// Store is a thread-safe in-memory key-value store.
// It now stores VersionedValue objects instead of raw strings.
type Store struct {
	mu   sync.RWMutex
	data map[string]VersionedValue
}

// NewStore initializes and returns a new empty Store.
func NewStore() *Store {
	return &Store{
		data: make(map[string]VersionedValue),
	}
}

// Set adds or updates a key-value pair.
// Crucially, it increments the version number on every write.
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment version, even for new keys (starts at version 1).
	current, _ := s.data[key]
	s.data[key] = VersionedValue{
		Value:   value,
		Version: current.Version + 1,
	}
}

// Get retrieves a VersionedValue for a given key.
// It now returns the full struct, not just the string value.
func (s *Store) Get(key string) (VersionedValue, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	return value, ok
}

// Delete removes a key-value pair from the store.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}