// Package config handles loading and parsing the application's configuration.
package config

import "github.com/BurntSushi/toml"

// Config holds all configuration for the application.
// We use struct tags to explicitly map TOML keys to struct fields.
type Config struct {
	NodeID   string   `toml:"node_id"`    // Unique ID for the node in the cluster
	Host     string   `toml:"host"`
	Port     int      `toml:"port"`
	RaftPort int      `toml:"raft_port"`  // Port for Raft's internal communication
	DataDir  string   `toml:"data_dir"`   // Directory to store Raft's data
	Peers    []string `toml:"peers"`      // List of other node IDs in the cluster
}

// New returns a new Config with default values.
func New() *Config {
    return &Config{
        NodeID:   "",
        Host:     "localhost",
        Port:     8080,
        RaftPort: 9080,
        DataDir:  ".",
        Peers:    []string{},
    }
}

// Load reads a configuration file from the given path and populates the Config struct.
func (c *Config) Load(path string) error {
	_, err := toml.DecodeFile(path, c)
	return err
}