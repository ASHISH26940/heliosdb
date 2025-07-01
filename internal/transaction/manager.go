// Package transaction manages the lifecycle of database transactions.
package transaction

import (
	"sync"
	"github.com/google/uuid"
)

// ReadOp represents a key that was read during a transaction, and its version at the time of reading.
type ReadOp struct {
	Key     string
	Version uint64
}

// WriteOp represents a key-value pair that will be written upon commit.
type WriteOp struct {
	Key   string
	Value string
}

// Transaction holds the state for a single, in-flight transaction.
type Transaction struct {
	ID       string
	ReadSet  []ReadOp
	WriteSet []WriteOp
}

// Manager is a thread-safe manager for all active transactions.
type Manager struct {
	mu           sync.RWMutex
	transactions map[string]*Transaction
}

// NewManager creates a new transaction manager.
func NewManager() *Manager {
	return &Manager{
		transactions: make(map[string]*Transaction),
	}
}

// Begin starts a new transaction and returns its unique ID.
func (m *Manager) Begin() *Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx := &Transaction{
		ID:       uuid.NewString(), // Generate a unique ID
		ReadSet:  make([]ReadOp, 0),
		WriteSet: make([]WriteOp, 0),
	}
	m.transactions[tx.ID] = tx
	return tx
}

// Get retrieves an active transaction by its ID.
func (m *Manager) Get(txID string) (*Transaction, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tx, ok := m.transactions[txID]
	return tx, ok
}

// StageWrite adds a write operation to a transaction's write set.
func (t *Transaction) StageWrite(key, value string) {
	t.WriteSet = append(t.WriteSet, WriteOp{Key: key, Value: value})
}

// StageRead adds a read operation to a transaction's read set.
func (t *Transaction) StageRead(key string, version uint64) {
	t.ReadSet = append(t.ReadSet, ReadOp{Key: key, Version: version})
}

// Clear removes a transaction from the manager, usually after a commit or rollback.
func (m *Manager) Clear(txID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.transactions, txID)
}