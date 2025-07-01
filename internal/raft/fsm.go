// Package raft contains the implementation of the Raft consensus layer.
package raft

import (
	"encoding/json"
	"io"
	"log"

	"github.com/ASHISH26940/heliosdb/internal/persistence"
	"github.com/ASHISH26940/heliosdb/internal/store"
	"github.com/ASHISH26940/heliosdb/internal/transaction"
	"github.com/hashicorp/raft"
)

// DataStore is the interface our FSM needs to interact with the storage layer.
type DataStore interface {
	Get(key string) (store.VersionedValue, bool)
	Set(key, value string)
	Delete(key string)
}

// Command is updated to handle both simple operations and transactional commits.
type Command struct {
	Op       string                  `json:"op"`
	Key      string                  `json:"key,omitempty"`
	Value    string                  `json:"value,omitempty"`
	WriteSet []transaction.WriteOp `json:"write_set,omitempty"` // For transactions
}

// FSM is a Finite State Machine that applies Raft logs to the key-value store.
type FSM struct {
	store DataStore
	wal   *persistence.WAL
}

// NewFSM creates a new FSM with a given data store and WAL.
func NewFSM(store DataStore, wal *persistence.WAL) *FSM {
	return &FSM{
		store: store,
		wal:   wal,
	}
}

// Apply applies a Raft log entry to the key-value store AFTER writing it to the WAL.
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(logEntry.Data, &cmd); err != nil {
		log.Panicf("Failed to unmarshal command: %v", err)
	}

	if err := f.wal.WriteCommand(cmd); err != nil {
		log.Panicf("Failed to write command to WAL: %v", err)
	}

	log.Printf("FSM: Applying command: %+v", cmd)

	switch cmd.Op {
	case "SET":
		f.store.Set(cmd.Key, cmd.Value)
	case "DELETE":
		f.store.Delete(cmd.Key)
	case "TX_COMMIT":
		// For a transaction, apply all writes in the write set.
		for _, op := range cmd.WriteSet {
			f.store.Set(op.Key, op.Value)
		}
	default:
		log.Printf("FSM: Unrecognized command op: %s", cmd.Op)
	}

	return nil
}

// Snapshot is used to support log compaction.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil // Not implemented in this phase
}

// Restore is used to restore an FSM from a snapshot.
func (f *FSM) Restore(rc io.ReadCloser) error {
	return nil // Not implemented in this phase
}