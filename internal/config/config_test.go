// Package config_test contains the unit tests for the config package.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Load(t *testing.T) {
	// Create a temporary directory for our test config files
	tempDir := t.TempDir()

	// --- Test Case 1: Valid configuration file ---
	validToml := `
host = "127.0.0.1"
port = 9000
peers = ["http://localhost:9001", "http://localhost:9002"]
`
	validPath := filepath.Join(tempDir, "valid.toml")
	if err := os.WriteFile(validPath, []byte(validToml), 0644); err != nil {
		t.Fatalf("failed to write valid config file: %v", err)
	}

	cfg := New()
	err := cfg.Load(validPath)
	if err != nil {
		t.Fatalf("expected no error loading valid config, but got: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host to be '127.0.0.1', but got '%s'", cfg.Host)
	}
	if cfg.Port != 9000 {
		t.Errorf("expected port to be 9000, but got %d", cfg.Port)
	}
	if len(cfg.Peers) != 2 || cfg.Peers[0] != "http://localhost:9001" {
		t.Errorf("peers were not parsed correctly")
	}

	// --- Test Case 2: File does not exist ---
	cfg2 := New()
	err = cfg2.Load(filepath.Join(tempDir, "nonexistent.toml"))
	if err == nil {
		t.Fatal("expected an error for non-existent file, but got none")
	}

	// --- Test Case 3: Invalid TOML format ---
	invalidToml := `host = 127.0.0.1` // Invalid: host should be a string
	invalidPath := filepath.Join(tempDir, "invalid.toml")
	if err := os.WriteFile(invalidPath, []byte(invalidToml), 0644); err != nil {
		t.Fatalf("failed to write invalid config file: %v", err)
	}

	cfg3 := New()
	err = cfg3.Load(invalidPath)
	if err == nil {
		t.Fatal("expected an error for invalid TOML, but got none")
	}
}