// Package transaction_test contains the unit tests for the transaction package.
package transaction

import "testing"

func TestManager(t *testing.T) {
	m := NewManager()

	// 1. Begin a new transaction
	tx1 := m.Begin()
	if tx1.ID == "" {
		t.Fatal("expected a transaction ID, but it was empty")
	}

	// 2. Verify we can get the transaction
	retrievedTx, ok := m.Get(tx1.ID)
	if !ok {
		t.Fatalf("expected to retrieve transaction %s, but it was not found", tx1.ID)
	}
	if retrievedTx.ID != tx1.ID {
		t.Errorf("retrieved transaction ID does not match")
	}

	// 3. Stage some operations
	retrievedTx.StageRead("key1", 5)
	retrievedTx.StageWrite("key2", "value2")

	if len(retrievedTx.ReadSet) != 1 || retrievedTx.ReadSet[0].Key != "key1" {
		t.Error("failed to stage a read operation correctly")
	}
	if len(retrievedTx.WriteSet) != 1 || retrievedTx.WriteSet[0].Value != "value2" {
		t.Error("failed to stage a write operation correctly")
	}

	// 4. Begin another transaction to ensure IDs are unique
	tx2 := m.Begin()
	if tx1.ID == tx2.ID {
		t.Fatal("expected transaction IDs to be unique")
	}

	// 5. Clear the first transaction
	m.Clear(tx1.ID)
	_, ok = m.Get(tx1.ID)
	if ok {
		t.Errorf("expected transaction %s to be cleared, but it still exists", tx1.ID)
	}

	// 6. Ensure the second transaction still exists
	_, ok = m.Get(tx2.ID)
	if !ok {
		t.Errorf("transaction %s was cleared unexpectedly", tx2.ID)
	}
}