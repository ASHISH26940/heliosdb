// Package store_test contains the unit tests for the store package.
package store

import (
	"fmt"
	"sync"
	"testing"
)

// TestStore_Versioning tests the basic lifecycle and version incrementing.
func TestStore_Versioning(t *testing.T) {
	s := NewStore()
	key := "test_key"
	value1 := "value1"
	value2 := "value2"

	// 1. Get a non-existent key
	_, ok := s.Get(key)
	if ok {
		t.Errorf("expected key '%s' to not exist, but it does", key)
	}

	// 2. Set a new key, version should be 1
	s.Set(key, value1)
	retrieved, ok := s.Get(key)
	if !ok {
		t.Errorf("expected key '%s' to exist, but it does not", key)
	}
	if retrieved.Value != value1 {
		t.Errorf("expected value '%s', but got '%s'", value1, retrieved.Value)
	}
	if retrieved.Version != 1 {
		t.Errorf("expected version to be 1, but got %d", retrieved.Version)
	}

	// 3. Update the key, version should be 2
	s.Set(key, value2)
	retrieved, ok = s.Get(key)
	if !ok {
		t.Errorf("expected key '%s' to still exist, but it does not", key)
	}
	if retrieved.Value != value2 {
		t.Errorf("expected updated value '%s', but got '%s'", value2, retrieved.Value)
	}
	if retrieved.Version != 2 {
		t.Errorf("expected version to be 2, but got %d", retrieved.Version)
	}

	// 4. Delete the key
	s.Delete(key)
	_, ok = s.Get(key)
	if ok {
		t.Errorf("expected key '%s' to be deleted, but it still exists", key)
	}
}

// TestStore_Concurrency race condition test remains largely the same.
func TestStore_Concurrency(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	s.Set("initial_key", "initial_value")

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", goroutineID, j)
				if j%2 == 0 {
					s.Set(key, "some_value")
				} else {
					s.Get("initial_key")
				}
			}
		}(i)
	}
	wg.Wait()
}