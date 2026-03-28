package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// MigrateRuntimeDB copies the unified runtime database to a new location.
func MigrateRuntimeDB(oldPath, newPath string) error {
	oldRuntimePath := strings.TrimSpace(oldPath)
	newRuntimePath := strings.TrimSpace(newPath)
	if oldRuntimePath == "" {
		return fmt.Errorf("old runtime database path is empty")
	}
	if newRuntimePath == "" {
		return fmt.Errorf("new runtime database path is empty")
	}
	if oldRuntimePath == newRuntimePath {
		return nil
	}

	oldInfo, err := os.Stat(oldRuntimePath)
	if err != nil {
		if os.IsNotExist(err) {
			f, createErr := os.OpenFile(newRuntimePath, os.O_RDWR|os.O_CREATE, 0o644)
			if createErr != nil {
				return fmt.Errorf("create new runtime database file: %w", createErr)
			}
			if closeErr := f.Close(); closeErr != nil {
				return fmt.Errorf("close new runtime database file: %w", closeErr)
			}
			return nil
		}
		return fmt.Errorf("stat old runtime database: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(newRuntimePath), 0o755); err != nil {
		return fmt.Errorf("create new runtime database directory: %w", err)
	}
	if newInfo, err := os.Stat(newRuntimePath); err == nil && newInfo.Size() > 0 {
		return fmt.Errorf("target runtime database already exists: %s", newRuntimePath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat new runtime database: %w", err)
	}

	src, err := os.Open(oldRuntimePath)
	if err != nil {
		return fmt.Errorf("open old runtime database: %w", err)
	}
	defer func() {
		_ = src.Close()
	}()

	tmpPath := filepath.Join(
		filepath.Dir(newRuntimePath),
		fmt.Sprintf(".tmp-runtime-db-%d-%d", os.Getpid(), time.Now().UnixNano()),
	)
	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, oldInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create temp runtime database: %w", err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = dst.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy runtime database: %w", err)
	}
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync runtime database copy: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close temp runtime database: %w", err)
	}
	if err := os.Rename(tmpPath, newRuntimePath); err != nil {
		return fmt.Errorf("move migrated runtime database: %w", err)
	}
	if dirFile, err := os.Open(filepath.Dir(newRuntimePath)); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	cleanup = false
	return nil
}
