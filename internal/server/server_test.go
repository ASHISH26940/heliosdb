// Package server_test contains the unit tests for the server package.
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"github.com/hashicorp/raft"
)

// mockStore is a mock implementation of the DataStore interface for testing.
type mockStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func newMockStore() *mockStore {
	return &mockStore{
		data: make(map[string]string),
	}
}

func (m *mockStore) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok
}

func (m *mockStore) Set(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *mockStore) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

func TestKVHandlers(t *testing.T) {
	store := newMockStore()
	// Create a mock or nil *raft.Raft as required by New
	var mockRaft *raft.Raft = nil // Replace with a proper mock if needed
	server := New(store, mockRaft)

	// --- Test Case 1: Set a new key ---
	body := `{"value":"bar"}`
	req := httptest.NewRequest(http.MethodPost, "/kv/foo", strings.NewReader(body))
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	// Verify the data was stored in our mock store
	val, ok := store.Get("foo")
	if !ok || val != "bar" {
		t.Errorf("expected key 'foo' to be set to 'bar', but it was not")
	}

	// --- Test Case 2: Get the key ---
	req = httptest.NewRequest(http.MethodGet, "/kv/foo", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.String() != "bar" {
		t.Errorf("expected body to be 'bar', but got '%s'", rr.Body.String())
	}

	// --- Test Case 3: Get a non-existent key ---
	req = httptest.NewRequest(http.MethodGet, "/kv/baz", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	// --- Test Case 4: Delete the key ---
	req = httptest.NewRequest(http.MethodDelete, "/kv/foo", nil)
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify the key is gone from the mock store
	_, ok = store.Get("foo")
	if ok {
		t.Error("expected key 'foo' to be deleted, but it still exists")
	}
}