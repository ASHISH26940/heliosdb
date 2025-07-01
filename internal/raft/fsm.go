package raft

import (
	"encoding/json"
	"io"
	"log"
	"github.com/hashicorp/raft"
	"github.com/ASHISH26940/heliosdb/internal/persistence"
)

type DataStore interface {
	Get(key string) (string, bool)
	Set(key, value string)
	Delete(key string)
}

type Command struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type FSM struct{
	store DataStore
	wal *persistence.WAL
}

func NewFSM(store DataStore,wal *persistence.WAL) *FSM{
	return &FSM{
		store: store,
		wal: wal,
	}
}

func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(logEntry.Data, &cmd); err != nil {
		// Using log.Panicf is a common pattern in FSMs. If the log entry is
		// corrupt, the node is in an unrecoverable state.
		log.Panicf("Failed to unmarshal command: %v", err)
	}

	if err:=f.wal.WriteCommand(cmd);err!=nil{
		log.Panicf("Failed to write command to WAL: %v", err)
	}

	log.Printf("FSM: Applying command: %+v", cmd)

	switch cmd.Op {
	case "SET":
		f.store.Set(cmd.Key, cmd.Value)
	case "DELETE":
		f.store.Delete(cmd.Key)
	default:
		log.Panicf("Unrecognized command op: %s", cmd.Op)
	}

	return nil
}

// Snapshot is used to support log compaction. It returns an FSM-specific
// snapshot object that can be used to restore the FSM's state.
// For now, we will not implement this.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil // Implementation left for a future stage
}

// Restore is used to restore an FSM from a snapshot. It is called when
// a node is bringing up a new FSM and has a snapshot to apply.
// For now, we will not implement this.
func (f *FSM) Restore(rc io.ReadCloser) error {
	return nil // Implementation left for a future stage
}