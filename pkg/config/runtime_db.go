package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// RuntimeDBName is the unified SQLite filename used by all runtime modules.
	RuntimeDBName = "nekobot.db"
)

// RuntimeDBPath returns the unified runtime database path.
func RuntimeDBPath(cfg *Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is nil")
	}

	dbDir := strings.TrimSpace(cfg.DatabaseDir())
	if dbDir == "" {
		return "", fmt.Errorf("database directory is empty")
	}
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return "", fmt.Errorf("create database directory: %w", err)
	}

	return filepath.Join(dbDir, RuntimeDBName), nil
}

// EnsureRuntimeDBFile ensures the runtime database file exists and returns its path.
func EnsureRuntimeDBFile(cfg *Config) (string, error) {
	dbPath, err := RuntimeDBPath(cfg)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return "", fmt.Errorf("create runtime database file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close runtime database file: %w", err)
	}
	return dbPath, nil
}
