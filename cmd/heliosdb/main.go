// Package main is the entry point for the HeliosDB server application.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ASHISH26940/heliosdb/internal/config"
	"github.com/ASHISH26940/heliosdb/internal/persistence"
	internal_raft "github.com/ASHISH26940/heliosdb/internal/raft"
	"github.com/ASHISH26940/heliosdb/internal/server"
	"github.com/ASHISH26940/heliosdb/internal/store"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
)

func main() {
	// --- Configuration and Flags ---
	configFile := flag.String("config", "config.toml", "Path to config file")
	bootstrap := flag.Bool("bootstrap", false, "Bootstrap the cluster (run on the first node only)")
	flag.Parse()

	cfg := config.New()
	if err := cfg.Load(*configFile); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// --- Initialize Store and Restore from WAL ---
	st := store.NewStore()
	walPath := filepath.Join(cfg.DataDir, "app.wal")
	log.Printf("Replaying Write-Ahead Log from %s...", walPath)

	err := persistence.Replay(walPath, func(cmdBytes []byte) error {
		var cmd internal_raft.Command
		if err := json.Unmarshal(cmdBytes, &cmd); err != nil {
			return err
		}
		switch cmd.Op {
		case "SET":
			st.Set(cmd.Key, cmd.Value)
		case "DELETE":
			st.Delete(cmd.Key)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to replay WAL: %v", err)
	}
	log.Println("WAL replay complete. Store is up to date.")

	// --- Open WAL for new commands ---
	wal, err := persistence.NewWAL(walPath)
	if err != nil {
		log.Fatalf("Failed to open WAL: %v", err)
	}

	// --- Initialize FSM with Store and WAL ---
	fsm := internal_raft.NewFSM(st, wal)

	// --- Raft Setup ---
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(cfg.NodeID)

	raftAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.RaftPort)
	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		log.Fatalf("Failed to resolve Raft address: %v", err)
	}
	transport, err := raft.NewTCPTransport(raftAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Fatalf("Failed to create Raft transport: %v", err)
	}

	snapshots, err := raft.NewFileSnapshotStore(cfg.DataDir, 2, os.Stderr)
	if err != nil {
		log.Fatalf("Failed to create snapshot store: %v", err)
	}

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(cfg.DataDir, "raft.db"))
	if err != nil {
		log.Fatalf("Failed to create bolt store: %v", err)
	}

	r, err := raft.NewRaft(raftConfig, fsm, logStore, logStore, snapshots, transport)
	if err != nil {
		log.Fatalf("Failed to create raft node: %v", err)
	}

	// --- Conditionally Bootstrap the Cluster ---
	if *bootstrap {
		log.Println("Bootstrapping cluster...")
		bootstrapConfig := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(cfg.NodeID),
					Address: transport.LocalAddr(),
				},
			},
		}
		r.BootstrapCluster(bootstrapConfig)
	}

	// --- Start the HTTP Server ---
	httpServer := server.New(st, r)
	httpAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("Starting HTTP server on %s", httpAddr)
	go func() {
		if err := http.ListenAndServe(httpAddr, httpServer); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	log.Println("HeliosDB node started successfully.")
	select {}
}