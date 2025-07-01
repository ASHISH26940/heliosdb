// Package server handles the HTTP API for the key-value store.
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	v1 "github.com/ASHISH26940/heliosdb/api/v1"
	"github.com/ASHISH26940/heliosdb/internal/store"
	"github.com/ASHISH26940/heliosdb/internal/transaction"
	"github.com/hashicorp/raft"
)

// DataStore is the interface our server needs to interact with the storage layer.
type DataStore interface {
	Get(key string) (store.VersionedValue, bool)
	Set(key, value string)
	Delete(key string)
}

// RaftNode is the interface our server needs to interact with the Raft layer.
type RaftNode interface {
	State() raft.RaftState
	Leader() raft.ServerAddress
	Apply(cmd []byte, timeout time.Duration) raft.ApplyFuture
	AddVoter(id raft.ServerID, address raft.ServerAddress, prevIndex uint64, timeout time.Duration) raft.IndexFuture
}

// Command represents a single command that will be committed to the Raft log.
type Command struct {
	Op       string                  `json:"op"`
	Key      string                  `json:"key,omitempty"`
	Value    string                  `json:"value,omitempty"`
	WriteSet []transaction.WriteOp `json:"write_set,omitempty"`
}

// Server now holds a transaction manager.
type Server struct {
	store  DataStore
	raft   RaftNode
	txm    *transaction.Manager // Transaction Manager
	router *http.ServeMux
}

// New is updated to initialize and accept the transaction manager.
func New(store DataStore, r RaftNode) *Server {
	s := &Server{
		store:  store,
		raft:   r,
		txm:    transaction.NewManager(), // Initialize the manager
		router: http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// ServeHTTP makes our Server a standard http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	s.router.HandleFunc("/kv/", s.handleKV)
	s.router.HandleFunc("/join", s.handleJoin)
	// Add new routes for transactions
	s.router.HandleFunc("/tx/begin", s.handleTxBegin)
	s.router.HandleFunc("/tx/set", s.handleTxSet)
	s.router.HandleFunc("/tx/commit", s.handleTxCommit)
}

// --- NEW TRANSACTION HANDLERS ---

func (s *Server) handleTxBegin(w http.ResponseWriter, r *http.Request) {
	tx := s.txm.Begin()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"tx_id": tx.ID})
}

func (s *Server) handleTxSet(w http.ResponseWriter, r *http.Request) {
	txID := r.URL.Query().Get("tx_id")
	key := r.URL.Query().Get("key")

	tx, ok := s.txm.Get(txID)
	if !ok {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	var req v1.SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tx.StageWrite(key, req.Value)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTxCommit(w http.ResponseWriter, r *http.Request) {
	if s.raft.State() != raft.Leader {
		http.Error(w, "Commits must be sent to the leader node", http.StatusForbidden)
		return
	}

	txID := r.URL.Query().Get("tx_id")
	tx, ok := s.txm.Get(txID)
	if !ok {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}
	defer s.txm.Clear(txID)

	// NOTE: A real OCC implementation would check the transaction's read-set
	// against the store's current versions here before committing.
	// We are simplifying this step for the example.

	cmd := Command{
		Op:       "TX_COMMIT",
		WriteSet: tx.WriteSet,
	}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		http.Error(w, "Failed to marshal command", http.StatusInternalServerError)
		return
	}

	future := s.raft.Apply(cmdBytes, 5*time.Second)
	if err := future.Error(); err != nil {
		http.Error(w, "Failed to apply transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// --- EXISTING HANDLERS ---

// handleJoin adds a new node to the Raft cluster.
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if s.raft.State() != raft.Leader {
		http.Error(w, "Can only join a cluster via the leader node", http.StatusForbidden)
		return
	}

	var joinReq struct {
		NodeID string `json:"node_id"`
		Addr   string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&joinReq); err != nil {
		http.Error(w, "Invalid join request body", http.StatusBadRequest)
		return
	}

	if joinReq.NodeID == "" || joinReq.Addr == "" {
		http.Error(w, "Missing node_id or addr in join request", http.StatusBadRequest)
		return
	}

	log.Printf("LEADER: Received join request for node %s at %s", joinReq.NodeID, joinReq.Addr)

	// Use the correct Raft command to add a new voter.
	future := s.raft.AddVoter(raft.ServerID(joinReq.NodeID), raft.ServerAddress(joinReq.Addr), 0, 0)
	if err := future.Error(); err != nil {
		log.Printf("LEADER: Failed to add voter: %v", err)
		http.Error(w, "Failed to add node to cluster: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("LEADER: Successfully added node %s to the cluster", joinReq.NodeID)
	w.WriteHeader(http.StatusOK)
}

// handleKV is the main dispatcher for all /kv/ requests.
func (s *Server) handleKV(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/kv/")
	if key == "" {
		http.Error(w, "Key is missing", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost || r.Method == http.MethodDelete {
		if s.raft.State() != raft.Leader {
			leaderAddr := string(s.raft.Leader())
			http.Error(w, "Writes must be sent to the leader at: "+leaderAddr, http.StatusForbidden)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, r, key)
	case http.MethodPost:
		s.handleSet(w, r, key)
	case http.MethodDelete:
		s.handleDelete(w, r, key)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGet serves read requests.
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, key string) {
	vv, ok := s.store.Get(key)
	if !ok {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(vv.Value + "\n"))
}

// handleSet serves write requests.
func (s *Server) handleSet(w http.ResponseWriter, r *http.Request, key string) {
	var req v1.SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cmd := Command{
		Op:    "SET",
		Key:   key,
		Value: req.Value,
	}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		http.Error(w, "Failed to marshal command", http.StatusInternalServerError)
		return
	}

	future := s.raft.Apply(cmdBytes, 5*time.Second)
	if err := future.Error(); err != nil {
		http.Error(w, "Failed to apply command: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Applied 'SET' for key '%s' via Raft", key)
	w.WriteHeader(http.StatusCreated)
}

// handleDelete serves delete requests.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, key string) {
	cmd := Command{
		Op:  "DELETE",
		Key: key,
	}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		http.Error(w, "Failed to marshal command", http.StatusInternalServerError)
		return
	}

	future := s.raft.Apply(cmdBytes, 5*time.Second)
	if err := future.Error(); err != nil {
		http.Error(w, "Failed to apply command: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Applied 'DELETE' for key '%s' via Raft", key)
	w.WriteHeader(http.StatusOK)
}