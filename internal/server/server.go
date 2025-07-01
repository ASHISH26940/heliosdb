// Package server handles the HTTP API for the key-value store.
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	v1 "github.com/ASHISH26940/heliosdb/api/v1"
	"github.com/hashicorp/raft"
)

// DataStore is the interface our server needs to interact with the storage layer.
// By depending on an interface, we can easily mock the store in our tests.
type DataStore interface {
	Get(key string) (string, bool)
	Set(key, value string)
	Delete(key string)
}

// Command represents a single command that will be committed to the Raft log.
type Command struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// Server is the HTTP server for our key-value store.
// It holds a reference to the Raft node to manage leadership and data replication.
type Server struct {
	store  DataStore
	raft   *raft.Raft
	router *http.ServeMux
}

// New creates a new Server instance.
func New(store DataStore, r *raft.Raft) *Server {
	s := &Server{
		store:  store,
		raft:   r,
		router: http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// ServeHTTP makes our Server a standard http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// registerRoutes sets up the HTTP routing for the server.
func (s *Server) registerRoutes() {
    s.router.HandleFunc("/kv/", s.handleKV)
    s.router.HandleFunc("/join", s.handleJoin) // <-- ADD THIS LINE
}

// handleKV is the main dispatcher for all /kv/ requests.
func (s *Server) handleKV(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/kv/")
	if key == "" {
		http.Error(w, "Key is missing", http.StatusBadRequest)
		return
	}

	// For write operations, we must check for leadership first.
	// Reads can be served by any node (eventual consistency).
	if r.Method == http.MethodPost || r.Method == http.MethodDelete {
		if s.raft.State() != raft.Leader {
			// In a production system, you might proxy the request to the leader.
			// For simplicity, we just return an error and the leader's address.
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

// handleGet serves read requests. It reads directly from the local store.
// This means followers might have slightly stale data (eventual consistency).
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, key string) {
	value, ok := s.store.Get(key)
	if !ok {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(value + "\n"))
}

// handleSet serves write requests. It submits a "SET" command to the Raft log.
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

	// Apply the command to the Raft log.
	// This is a blocking call that waits until the command is committed
	// by a majority of the cluster and applied to the FSM.
	future := s.raft.Apply(cmdBytes, 5*time.Second)
	if err := future.Error(); err != nil {
		http.Error(w, "Failed to apply command: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Applied 'SET' for key '%s' via Raft", key)
	w.WriteHeader(http.StatusCreated)
}

// handleDelete serves delete requests. It submits a "DELETE" command to the Raft log.
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

    // Add the new node to the cluster as a voter.
    future := s.raft.AddVoter(raft.ServerID(joinReq.NodeID), raft.ServerAddress(joinReq.Addr), 0, 0)
    if err := future.Error(); err != nil {
        log.Printf("LEADER: Failed to add voter: %v", err)
        http.Error(w, "Failed to add node to cluster: "+err.Error(), http.StatusInternalServerError)
        return
    }

    log.Printf("LEADER: Successfully added node %s to the cluster", joinReq.NodeID)
    w.WriteHeader(http.StatusOK)
}