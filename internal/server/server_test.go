// Package server_test contains the unit tests for the server package.
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ASHISH26940/heliosdb/internal/store"
	"github.com/hashicorp/raft"
)

// mockStore is a mock implementation of the DataStore interface for testing.
type mockStore struct {
	mu   sync.RWMutex
	data map[string]store.VersionedValue
}

func newMockStore() *mockStore {
	return &mockStore{
		data: make(map[string]store.VersionedValue),
	}
}

func (m *mockStore) Get(key string) (store.VersionedValue, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok
}

func (m *mockStore) Set(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, _ := m.data[key]
	m.data[key] = store.VersionedValue{
		Value:   value,
		Version: current.Version + 1,
	}
}

func (m *mockStore) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// --- Updated Mock Raft Implementation ---

type mockApplyFuture struct{}

func (m *mockApplyFuture) Error() error        { return nil }
func (m *mockApplyFuture) Response() interface{} { return nil }
func (m *mockApplyFuture) Index() uint64       { return 0 }
func (m *mockApplyFuture) Done() <-chan struct{} { return nil }

// mockRaft now holds a reference to the mockStore to simulate the FSM's behavior.
type mockRaft struct {
	isLeader bool
	store    *mockStore // Reference to the mock store
}

// AddVoter is a mock implementation to satisfy the RaftNode interface.
func (m *mockRaft) AddVoter(id raft.ServerID, address raft.ServerAddress, prevIndex uint64, timeout time.Duration) raft.IndexFuture {
	return &mockIndexFuture{}
}

// mockIndexFuture is a mock implementation of raft.IndexFuture.
type mockIndexFuture struct{}

func (m *mockIndexFuture) Error() error        { return nil }
func (m *mockIndexFuture) Index() uint64       { return 0 }
func (m *mockIndexFuture) Response() interface{} { return nil }
func (m *mockIndexFuture) Done() <-chan struct{} { return nil }

func (m *mockRaft) State() raft.RaftState {
	if m.isLeader {
		return raft.Leader
	}
	return raft.Follower
}
func (m *mockRaft) Leader() raft.ServerAddress { return "localhost:8080" }

// Apply now decodes the command and updates the mockStore, mimicking the FSM.
func (m *mockRaft) Apply(cmdBytes []byte, timeout time.Duration) raft.ApplyFuture {
	var cmd Command
	if err := json.Unmarshal(cmdBytes, &cmd); err != nil {
		panic("failed to unmarshal command in mock raft")
	}

	switch cmd.Op {
	case "SET":
		m.store.Set(cmd.Key, cmd.Value)
	case "DELETE":
		m.store.Delete(cmd.Key)
	}

	return &mockApplyFuture{}
}

// --- Updated Test Function ---

func TestKVHandlers(t *testing.T) {
	store := newMockStore()
	// Create a mock Raft node that is aware of the mock store.
	mockRaftNode := &mockRaft{
		isLeader: true,
		store:    store, // Link the mock Raft to the mock store
	}
	// Pass both mocks to the server.
	srv := New(store, mockRaftNode)

	// --- Test Case 1: Set a new key ---
	body := `{"value":"bar"}`
	req := httptest.NewRequest(http.MethodPost, "/kv/foo", strings.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	// This assertion will now pass because mockRaft.Apply updated the store.
	val, ok := store.Get("foo")
	if !ok || val.Value != "bar" {
		t.Errorf("expected key 'foo' to be set to 'bar', but it was not")
	}

	// --- Test Case 2: Get the key ---
	req = httptest.NewRequest(http.MethodGet, "/kv/foo", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "bar" {
		t.Errorf("expected body to be 'bar', but got '%s'", rr.Body.String())
	}

	// --- Test Case 3: Get a non-existent key ---
	req = httptest.NewRequest(http.MethodGet, "/kv/baz", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	// --- Test Case 4: Delete the key ---
	req = httptest.NewRequest(http.MethodDelete, "/kv/foo", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify the key is gone from the mock store
	_, ok = store.Get("foo")
	if ok {
		t.Error("expected key 'foo' to be deleted, but it still exists")
	}
}