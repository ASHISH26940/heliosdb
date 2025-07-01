package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestStore_SetGetDelete(t *testing.T){
	s:=NewStore()
	key := "test_key"
	value1 := "value1"
	value2 := "value2"

	_,ok:=s.Get(key)
	if ok {
		t.Errorf("expected key '%s' to not exist, but it does", key)
	}

	// 2. Set a new key
	s.Set(key, value1)
	retrievedValue, ok := s.Get(key)
	if !ok {
		t.Errorf("expected key '%s' to exist, but it does not", key)
	}
	if retrievedValue != value1 {
		t.Errorf("expected value '%s', but got '%s'", value1, retrievedValue)
	}

	// 3. Update the key
	s.Set(key, value2)
	retrievedValue, ok = s.Get(key)
	if !ok {
		t.Errorf("expected key '%s' to still exist, but it does not", key)
	}
	if retrievedValue != value2 {
		t.Errorf("expected updated value '%s', but got '%s'", value2, retrievedValue)
	}

	// 4. Delete the key
	s.Delete(key)
	_, ok = s.Get(key)
	if ok {
		t.Errorf("expected key '%s' to be deleted, but it still exists", key)
	}
}

func TestStore_Concurrency(t *testing.T){
	s:=NewStore()
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	// Set an initial key to be read by goroutines
	s.Set("initial_key", "initial_value")

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Create a unique key for this operation to avoid simple overwrites
				key := fmt.Sprintf("key_%d_%d", goroutineID, j)
				
				// Perform a mix of operations
				if j%2 == 0 {
					// Write operation
					s.Set(key, "some_value")
				} else {
					// Read operation on the initial key
					s.Get("initial_key")
				}
			}
		}(i)
	}

	wg.Wait()

}